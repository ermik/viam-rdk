package gpio

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
)

const modelName = "gpio"

// init registers a pi motor based on pigpio.
func init() {

	x := registry.Motor{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (motor.Motor, error) {
		actualBoard, motorConfig, err := getBoardFromRobotConfig(r, config)
		if err != nil {
			return nil, err
		}

		m, err := NewMotor(actualBoard, *motorConfig, logger)
		if err != nil {
			return nil, err
		}

		m, err = WrapMotorWithEncoder(ctx, actualBoard, config, *motorConfig, m, logger)
		if err != nil {
			return nil, err
		}
		return m, nil
	}}

	registry.RegisterMotor(modelName, x)
	motor.RegisterConfigAttributeConverter(modelName)

	// For backwards compatibility
	registry.RegisterMotor("pi", x)
	motor.RegisterConfigAttributeConverter("pi")

}

func getBoardFromRobotConfig(r robot.Robot, config config.Component) (board.Board, *motor.Config, error) {
	motorConfig := config.ConvertedAttributes.(*motor.Config)
	if motorConfig.BoardName == "" {
		return nil, nil, errors.New("expected board name in config for motor")
	}
	b, ok := r.BoardByName(motorConfig.BoardName)
	if !ok {
		return nil, nil, fmt.Errorf("expected to find board %q", motorConfig.BoardName)
	}
	return b, motorConfig, nil
}