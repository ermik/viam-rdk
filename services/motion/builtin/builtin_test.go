package builtin_test

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/gripper"

	// register.
	commonpb "go.viam.com/api/common/v1"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/spatialmath"
)

func setupMotionServiceFromConfig(t *testing.T, configFilename string) motion.Service {
	t.Helper()
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(ctx, configFilename, logger)
	test.That(t, err, test.ShouldBeNil)
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer myRobot.Close(context.Background())
	svc, err := builtin.NewBuiltIn(ctx, myRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	return svc
}

func TestMoveFailures(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "../data/arm_gantry.json")
	t.Run("fail on not finding gripper", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("fakeCamera", spatialmath.NewPoseFromPoint(r3.Vector{10.0, 10.0, 10.0}))
		_, err = ms.Move(context.Background(), camera.Named("fake"), grabPose, &referenceframe.WorldState{}, map[string]interface{}{})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("fail on disconnected supplemental frames in world state", func(t *testing.T) {
		testPose := spatialmath.NewPoseFromOrientation(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
		)
		transforms := []*referenceframe.PoseInFrame{referenceframe.NewNamedPoseInFrame("noParent", testPose, "frame2")}
		worldState := &referenceframe.WorldState{Transforms: transforms}
		poseInFrame := referenceframe.NewPoseInFrame("frame2", spatialmath.NewZeroPose())
		_, err = ms.Move(context.Background(), arm.Named("arm1"), poseInFrame, worldState, map[string]interface{}{})
		test.That(t, err, test.ShouldBeError, framesystemparts.NewMissingParentError("frame2", "noParent"))
	})
}

func TestMove1(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "../data/moving_arm.json")

	t.Run("succeeds when all frame info in config", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{0, -30, -50}))
		_, err = ms.Move(context.Background(), gripper.Named("pieceGripper"), grabPose, &referenceframe.WorldState{}, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when mobile component can be solved for destinations in own frame", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("pieceArm", spatialmath.NewPoseFromPoint(r3.Vector{0, -30, -50}))
		_, err = ms.Move(context.Background(), gripper.Named("pieceArm"), grabPose, &referenceframe.WorldState{}, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when immobile component can be solved for destinations in own frame", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("pieceGripper", spatialmath.NewPoseFromPoint(r3.Vector{0, -30, -50}))
		_, err = ms.Move(context.Background(), gripper.Named("pieceGripper"), grabPose, &referenceframe.WorldState{}, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds with supplemental info in world state", func(t *testing.T) {
		testPose := spatialmath.NewPoseFromOrientation(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
		)

		transforms := []*referenceframe.PoseInFrame{
			referenceframe.NewNamedPoseInFrame(referenceframe.World, testPose, "testFrame2"),
			referenceframe.NewNamedPoseInFrame("pieceArm", testPose, "testFrame"),
		}

		worldState := &referenceframe.WorldState{Transforms: transforms}
		grabPose := referenceframe.NewPoseInFrame("testFrame2", spatialmath.NewPoseFromPoint(r3.Vector{-20, -130, -40}))
		_, err = ms.Move(context.Background(), gripper.Named("pieceGripper"), grabPose, worldState, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestMoveWithObstacles(t *testing.T) {
	ms := setupMotionServiceFromConfig(t, "../data/moving_arm.json")

	t.Run("check a movement that should not succeed due to obstacles", func(t *testing.T) {
		testPose1 := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 370})
		testPose2 := spatialmath.NewPoseFromPoint(r3.Vector{300, 300, -3500})
		_ = testPose2
		grabPose := referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{-600, -400, 460}))
		obsMsgs := []*commonpb.GeometriesInFrame{
			{
				ReferenceFrame: "world",
				Geometries: []*commonpb.Geometry{
					{
						Center: spatialmath.PoseToProtobuf(testPose2),
						GeometryType: &commonpb.Geometry_Box{
							Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{
								X: 20,
								Y: 40,
								Z: 40,
							}},
						},
					},
				},
			},
			{
				ReferenceFrame: "world",
				Geometries: []*commonpb.Geometry{
					{
						Center: spatialmath.PoseToProtobuf(testPose1),
						GeometryType: &commonpb.Geometry_Box{
							Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{
								X: 2000,
								Y: 2000,
								Z: 20,
							}},
						},
					},
				},
			},
		}
		worldState, err := referenceframe.WorldStateFromProtobuf(&commonpb.WorldState{Obstacles: obsMsgs})
		test.That(t, err, test.ShouldBeNil)
		_, err = ms.Move(context.Background(), gripper.Named("pieceArm"), grabPose, worldState, map[string]interface{}{})
		// This fails due to a large obstacle being in the way
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestMoveSingleComponent(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "../data/moving_arm.json")

	t.Run("succeeds when all frame info in config", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-25, 30, -200}))
		_, err = ms.MoveSingleComponent(
			context.Background(),
			arm.Named("pieceArm"),
			grabPose,
			&referenceframe.WorldState{},
			map[string]interface{}{},
		)
		// Gripper is not an arm and cannot move
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("fails due to gripper not being an arm", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-20, -30, -40}))
		_, err = ms.MoveSingleComponent(
			context.Background(),
			gripper.Named("pieceGripper"),
			grabPose,
			&referenceframe.WorldState{},
			map[string]interface{}{},
		)
		// Gripper is not an arm and cannot move
		test.That(t, err, test.ShouldNotBeNil)
	})

	ms = setupMotionServiceFromConfig(t, "../data/moving_arm.json")

	t.Run("succeeds with supplemental info in world state", func(t *testing.T) {
		testPose := spatialmath.NewPoseFromOrientation(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
		)
		transforms := []*referenceframe.PoseInFrame{referenceframe.NewNamedPoseInFrame(referenceframe.World, testPose, "testFrame2")}
		worldState := &referenceframe.WorldState{Transforms: transforms}

		poseToGrab := spatialmath.NewPoseFromOrientation(
			r3.Vector{X: -20., Y: 0., Z: -800.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 1., RY: 0., RZ: 0.},
		)

		grabPose := referenceframe.NewPoseInFrame("testFrame2", poseToGrab)
		_, err = ms.MoveSingleComponent(context.Background(), arm.Named("pieceArm"), grabPose, worldState, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestMultiplePieces(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "../data/fake_tomato.json")
	grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-0, -30, -50}))
	_, err = ms.Move(context.Background(), gripper.Named("gr"), grabPose, &referenceframe.WorldState{}, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
}

func TestGetPose(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "../data/arm_gantry.json")

	pose, err := ms.GetPose(context.Background(), arm.Named("gantry1"), "", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 1.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 501.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "gantry1", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 500)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = ms.GetPose(context.Background(), arm.Named("gantry1"), "gantry1", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "arm1", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, "arm1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	testPose := spatialmath.NewPoseFromOrientation(
		r3.Vector{X: 0., Y: 0., Z: 0.},
		&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
	)
	transforms := []*referenceframe.PoseInFrame{
		referenceframe.NewNamedPoseInFrame(referenceframe.World, testPose, "testFrame"),
		referenceframe.NewNamedPoseInFrame("testFrame", testPose, "testFrame2"),
	}

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "testFrame2", transforms, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, -501.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, -300)
	test.That(t, pose.Pose().Orientation().AxisAngles().RX, test.ShouldEqual, 0)
	test.That(t, pose.Pose().Orientation().AxisAngles().RY, test.ShouldEqual, -1)
	test.That(t, pose.Pose().Orientation().AxisAngles().RZ, test.ShouldEqual, 0)
	test.That(t, pose.Pose().Orientation().AxisAngles().Theta, test.ShouldAlmostEqual, math.Pi)

	transforms = []*referenceframe.PoseInFrame{referenceframe.NewNamedPoseInFrame("noParent", testPose, "testFrame")}
	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "testFrame", transforms, map[string]interface{}{})
	test.That(t, err, test.ShouldBeError, framesystemparts.NewMissingParentError("testFrame", "noParent"))
	test.That(t, pose, test.ShouldBeNil)
}
