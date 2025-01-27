// Package arm contains a gRPC based arm client.
package arm

import (
	"context"
	"errors"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/arm/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

var errArmClientModelNotValid = errors.New("unable to retrieve a valid arm model from arm client")

// client implements ArmServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.ArmServiceClient
	model  referenceframe.Model
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Arm {
	c := pb.NewArmServiceClient(conn)
	// TODO: DATA-853 requires that this support models being changed on the fly, not just at creation
	// TODO: RSDK-882 will update this so that this is not necessary
	r := robotpb.NewRobotServiceClient(conn)
	return &client{
		name:   name,
		conn:   conn,
		client: c,
		model:  getModel(ctx, r, name),
		logger: logger,
	}
}

func (c *client) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetEndPosition(ctx, &pb.GetEndPositionRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return spatialmath.NewPoseFromProtobuf(resp.Pose), nil
}

func (c *client) MoveToPosition(
	ctx context.Context,
	pose spatialmath.Pose,
	worldState *referenceframe.WorldState,
	extra map[string]interface{},
) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	worldStateMsg, err := referenceframe.WorldStateToProtobuf(worldState)
	if err != nil {
		return err
	}
	_, err = c.client.MoveToPosition(ctx, &pb.MoveToPositionRequest{
		Name:       c.name,
		To:         spatialmath.PoseToProtobuf(pose),
		WorldState: worldStateMsg,
		Extra:      ext,
	})
	return err
}

func (c *client) MoveToJointPositions(ctx context.Context, positions *pb.JointPositions, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.MoveToJointPositions(ctx, &pb.MoveToJointPositionsRequest{
		Name:      c.name,
		Positions: positions,
		Extra:     ext,
	})
	return err
}

func (c *client) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetJointPositions(ctx, &pb.GetJointPositionsRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return resp.Positions, nil
}

func (c *client) Stop(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Stop(ctx, &pb.StopRequest{
		Name:  c.name,
		Extra: ext,
	})
	return err
}

func (c *client) ModelFrame() referenceframe.Model {
	return c.model
}

func (c *client) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	resp, err := c.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	if c.model == nil {
		return nil, errArmClientModelNotValid
	}
	return c.model.InputFromProtobuf(resp), nil
}

func (c *client) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	if c.model == nil {
		return errArmClientModelNotValid
	}
	return c.MoveToJointPositions(ctx, c.model.ProtobufFromInput(goal), nil)
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

func getModel(ctx context.Context, r robotpb.RobotServiceClient, name string) referenceframe.Model {
	resp, err := r.FrameSystemConfig(ctx, &robotpb.FrameSystemConfigRequest{})
	if err != nil {
		return nil
	}
	cfgs := resp.GetFrameSystemConfigs()
	for _, cfg := range cfgs {
		if cfg.GetName() == name {
			if part, err := config.ProtobufToFrameSystemPart(cfg); err == nil {
				return part.ModelFrame
			}
			return nil
		}
	}
	return nil
}
