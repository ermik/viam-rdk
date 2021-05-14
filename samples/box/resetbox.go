package action

import (
	"context"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/core/action"
	"go.viam.com/core/board"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/robot"
	"go.viam.com/core/utils"
)

func init() {
	action.RegisterAction("ResetBox", ResetBox)
}

// ResetBox TODO
func ResetBox(ctx context.Context, theRobot robot.Robot) {
	err := ResetBoxSteps(ctx, theRobot, 4)
	if err != nil {
		theRobot.Logger().Errorf("error resetting box: %s", err)
	}
}

// ResetBoxSteps TODO
func ResetBoxSteps(ctx context.Context, theRobot robot.Robot, shakes int) error {
	resetBoard := theRobot.BoardByName("resetDriveBoard")
	if resetBoard == nil {
		return errors.New("robot does not have a resetDriveBoard")
	}
	// Dump object into the resetter
	err := OpenBox(ctx, resetBoard, true)
	if err != nil {
		return err
	}
	err = TiltField(ctx, resetBoard)
	if err != nil {
		return err
	}
	if !utils.SelectContextOrWait(ctx, 2*time.Second) {
		return ctx.Err()
	}
	err = FlatField(ctx, resetBoard)
	if err != nil {
		return err
	}

	// Shake the resetter the specified number of times
	for i := 0; i < shakes-1; i++ {
		err = CloseBox(ctx, resetBoard)
		if err != nil {
			return err
		}
		if !utils.SelectContextOrWait(ctx, 500*time.Millisecond) {
			return ctx.Err()
		}
		err = OpenBox(ctx, resetBoard, false)
		if err != nil {
			return err
		}
		if !utils.SelectContextOrWait(ctx, 500*time.Millisecond) {
			return ctx.Err()
		}
	}
	err = CloseBox(ctx, resetBoard)
	if err != nil {
		return err
	}
	if !utils.SelectContextOrWait(ctx, 500*time.Millisecond) {
		return ctx.Err()
	}
	err = OpenBox(ctx, resetBoard, true)
	if err != nil {
		return err
	}

	// Grab the object where it ought to be and replace it onto the field
	return ReplaceObject(ctx, theRobot)
}

// OpenBox TODO
func OpenBox(ctx context.Context, b board.Board, gentle bool) error {
	lSwitch := b.DigitalInterrupt("open")
	currentValue := lSwitch.Value()
	startValue := lSwitch.Value()

	// TODO(pl): decrease this once box is sturdier
	servoSpeed := 70

	// Gentle means we're just trying to open the box, not necessarily shake it
	if gentle {
		servoSpeed = 70
	}

	shakeServo := b.Servo("shake")

	// Back off in case we're already at the limit switch
	if gentle {
		err := shakeServo.Move(ctx, 100)
		if err != nil {
			return err
		}
		if !utils.SelectContextOrWait(ctx, 300*time.Millisecond) {
			return ctx.Err()
		}
	}
	err := shakeServo.Move(ctx, uint8(servoSpeed))
	if err != nil {
		return err
	}
	// Move until limit switch is hit
	for currentValue == startValue {
		currentValue = lSwitch.Value()
	}

	err = shakeServo.Move(ctx, 90)
	if err != nil {
		return err
	}
	return nil
}

// CloseBox TODO
func CloseBox(ctx context.Context, b board.Board) error {
	lSwitch := b.DigitalInterrupt("closed")
	currentValue := lSwitch.Value()
	startValue := lSwitch.Value()

	shakeServo := b.Servo("shake")
	err := shakeServo.Move(ctx, 110)
	if err != nil {
		return err
	}

	// Move until limit switch is hit
	for currentValue == startValue {
		currentValue = lSwitch.Value()
	}

	err = shakeServo.Move(ctx, 90)
	if err != nil {
		return err
	}
	return nil
}

// TiltField TODO
func TiltField(ctx context.Context, b board.Board) error {
	tiltServo := b.Servo("tilt")
	return tiltServo.Move(ctx, 70)
}

// FlatField TODO
func FlatField(ctx context.Context, b board.Board) error {
	tiltServo := b.Servo("tilt")
	return tiltServo.Move(ctx, 32)
}

// ReplaceObject TODO
func ReplaceObject(ctx context.Context, theRobot robot.Robot) error {
	myArm := theRobot.ArmByName("pieceArm")
	myGripper := theRobot.GripperByName("pieceGripper")
	err := myGripper.Open(ctx)
	if err != nil {
		return err
	}

	err = myArm.MoveToJointPositions(ctx, &pb.JointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}})
	if err != nil {
		return err
	}

	toDuckPositions := navigateWx250sToDuck()
	for _, intPosition := range toDuckPositions {
		err = myArm.MoveToJointPositions(ctx, intPosition)
		if err != nil {
			return err
		}
	}
	// TODO(pl): search pattern, additional shaking, etc if gripper grabs nothing
	_, err = myGripper.Grab(ctx)
	if err != nil {
		return err
	}

	if !utils.SelectContextOrWait(ctx, time.Second) {
		return ctx.Err()
	}
	fromDuckPositions := navigateWx250sFromDuck()
	for _, intPosition := range fromDuckPositions {
		err = myArm.MoveToJointPositions(ctx, intPosition)
		if err != nil {
			return err
		}
	}
	err = myGripper.Open(ctx)
	if err != nil {
		return err
	}

	if !utils.SelectContextOrWait(ctx, time.Second) {
		return ctx.Err()
	}
	err = myArm.MoveToJointPositions(ctx, &pb.JointPositions{Degrees: []float64{-86, 5, 5, 0, 70, 0}})
	if err != nil {
		return err
	}

	err = myArm.MoveToJointPositions(ctx, &pb.JointPositions{Degrees: []float64{-77, 0, 0, 0, 0, 0}})
	if err != nil {
		return err
	}

	return myArm.MoveToJointPositions(ctx, &pb.JointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}})
}

// TODO(pl) there's definitely a better way to script a series of recorded motions, but this works for now
func navigateWx250sToDuck() []*pb.JointPositions {
	var positions []*pb.JointPositions
	positions = append(positions, &pb.JointPositions{Degrees: []float64{-3.076171875, -101.42578125, 84.814453125, 2.724609375, 49.658203125, -11.6015625}},
		&pb.JointPositions{Degrees: []float64{-10.107421875, -89.6484375, 76.728515625, 2.021484375, 47.548828125, -8.0859375}},
		&pb.JointPositions{Degrees: []float64{-14.4140625, -80.5078125, 68.466796875, -3.779296875, 47.4609375, -7.734375}},
		&pb.JointPositions{Degrees: []float64{-17.841796875, -69.2578125, 61.787109375, -14.0625, 47.4609375, -7.55859375}},
		&pb.JointPositions{Degrees: []float64{-21.533203125, -54.404296875, 53.4375, -18.6328125, 47.548828125, -7.822265625}},
		&pb.JointPositions{Degrees: []float64{-23.73046875, -39.638671875, 44.6484375, -16.34765625, 47.63671875, -7.822265625}},
		&pb.JointPositions{Degrees: []float64{-28.388671875, -24.08203125, 31.9921875, -13.0078125, 50.09765625, -8.173828125}},
		&pb.JointPositions{Degrees: []float64{-31.728515625, -4.306640625, 17.2265625, -13.88671875, 53.349609375, -8.349609375}},
		&pb.JointPositions{Degrees: []float64{-34.98046875, 2.28515625, 16.34765625, -10.107421875, 55.01953125, -10.546875}},
		&pb.JointPositions{Degrees: []float64{-36.826171875, 7.822265625, 16.962890625, -9.66796875, 43.41796875, -13.095703125}},
		&pb.JointPositions{Degrees: []float64{-36.9140625, 17.666015625, 10.546875, -8.525390625, 39.990234375, -17.138671875}},
		&pb.JointPositions{Degrees: []float64{-38.408203125, 25.48828125, 7.3828125, -5.009765625, 35.595703125, -26.806640625}},
		&pb.JointPositions{Degrees: []float64{-38.49609375, 32.6953125, 2.4609375, -7.822265625, 35.595703125, -27.421875}},
		&pb.JointPositions{Degrees: []float64{-39.287109375, 40.25390625, -1.58203125, 1.494140625, 35.595703125, -27.59765625}},
		&pb.JointPositions{Degrees: []float64{-38.935546875, 44.82421875, -6.943359375, 1.93359375, 35.068359375, -27.94921875}},
		&pb.JointPositions{Degrees: []float64{-38.935546875, 48.515625, -7.470703125, -0.263671875, 32.8828125, -33.3984375}},
		&pb.JointPositions{Degrees: []float64{-38.583984375, 53.173828125, -12.568359375, 1.142578125, 30.443359375, -33.57421875}},
		&pb.JointPositions{Degrees: []float64{-38.49609375, 56.6015625, -14.765625, 1.845703125, 28.8828125, -33.75}},
		&pb.JointPositions{Degrees: []float64{-38.056640625, 58.7109375, -16.875, 1.40625, 28.41015625, -33.662109375}},
		&pb.JointPositions{Degrees: []float64{-38.232421875, 60.64453125, -18.28125, 1.845703125, 27.322265625, -33.662109375}},
		&pb.JointPositions{Degrees: []float64{-38.408203125, 62.2265625, -19.423828125, 1.494140625, 27.70703125, -33.486328125}})

	return positions
}

func navigateWx250sFromDuck() []*pb.JointPositions {
	var positions []*pb.JointPositions
	positions = append(positions, &pb.JointPositions{Degrees: []float64{-38.583984375, 62.138671875, -19.599609375, 2.63671875, 30.322265625, -33.486328125}},
		&pb.JointPositions{Degrees: []float64{-38.3203125, 59.150390625, -18.193359375, 2.373046875, 30.76171875, -33.134765625}},
		&pb.JointPositions{Degrees: []float64{-37.96875, 53.349609375, -14.94140625, 2.109375, 38.583984375, -32.958984375}},
		&pb.JointPositions{Degrees: []float64{-38.056640625, 42.36328125, -8.61328125, 2.373046875, 39.990234375, -33.046875}},
		&pb.JointPositions{Degrees: []float64{-38.14453125, 35.68359375, -2.197265625, 2.4609375, 39.990234375, -32.958984375}},
		&pb.JointPositions{Degrees: []float64{-38.3203125, 28.740234375, 0.87890625, 2.109375, 39.990234375, -32.958984375}},
		&pb.JointPositions{Degrees: []float64{-38.671875, 23.73046875, 1.0546875, 2.109375, 40.517578125, -32.87109375}},
		&pb.JointPositions{Degrees: []float64{-39.375, 19.6875, 2.109375, 2.373046875, 43.76953125, -32.87109375}},
		&pb.JointPositions{Degrees: []float64{-42.1875, 9.404296875, 5.712890625, 2.109375, 48.69140625, -32.87109375}},
		&pb.JointPositions{Degrees: []float64{-47.373046875, -0.439453125, 8.0859375, 2.021484375, 54.052734375, -32.783203125}},
		&pb.JointPositions{Degrees: []float64{-62.138671875, -9.84375, 8.525390625, -2.548828125, 60.380859375, -32.87109375}},
		&pb.JointPositions{Degrees: []float64{-71.806640625, -15.556640625, 10.37109375, -11.07421875, 63.017578125, -32.958984375}},
		&pb.JointPositions{Degrees: []float64{-79.98046875, -18.017578125, 13.18359375, -16.875, 65.0390625, -33.310546875}},
		&pb.JointPositions{Degrees: []float64{-86.8359375, -17.2265625, 17.666015625, -17.40234375, 60.46875, -13.18359375}},
		&pb.JointPositions{Degrees: []float64{-87.5390625, -13.53515625, 19.248046875, -4.04296875, 61.435546875, -4.921875}},
		&pb.JointPositions{Degrees: []float64{-87.71484375, -4.39453125, 17.9296875, -0.3515625, 63.017578125, -4.658203125}},
		&pb.JointPositions{Degrees: []float64{-87.5390625, 6.85546875, 9.931640625, -3.603515625, 68.203125, -4.39453125}},
		&pb.JointPositions{Degrees: []float64{-87.099609375, 9.66796875, 10.8984375, -0.439453125, 68.90625, -1.23046875}},
		&pb.JointPositions{Degrees: []float64{-86.484375, 10.1953125, 10.72265625, -0.791015625, 70.400390625, -0.263671875}})

	return positions
}