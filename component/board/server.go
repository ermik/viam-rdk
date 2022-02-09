// Package board contains a gRPC based Board service server.
package board

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the contract from board_subtype.proto.
type subtypeServer struct {
	pb.UnimplementedBoardServiceServer
	s subtype.Service
}

// NewServer constructs an board gRPC service server.
func NewServer(s subtype.Service) pb.BoardServiceServer {
	return &subtypeServer{s: s}
}

// getBoard returns the board specified, nil if not.
func (s *subtypeServer) getBoard(name string) (Board, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no board with name (%s)", name)
	}
	board, ok := resource.(Board)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a board", name)
	}
	return board, nil
}

// Status returns the status of a board of the underlying robot.
func (s *subtypeServer) Status(ctx context.Context, req *pb.BoardServiceStatusRequest) (*pb.BoardServiceStatusResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	status, err := b.Status(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.BoardServiceStatusResponse{Status: status}, nil
}

// SetGPIO sets a given pin of a board of the underlying robot to either low or high.
func (s *subtypeServer) SetGPIO(ctx context.Context, req *pb.BoardServiceSetGPIORequest) (*pb.BoardServiceSetGPIOResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.BoardServiceSetGPIOResponse{}, b.SetGPIO(ctx, req.Pin, req.High)
}

// GetGPIO gets the high/low state of a given pin of a board of the underlying robot.
func (s *subtypeServer) GetGPIO(ctx context.Context, req *pb.BoardServiceGetGPIORequest) (*pb.BoardServiceGetGPIOResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	high, err := b.GetGPIO(ctx, req.Pin)
	if err != nil {
		return nil, err
	}
	return &pb.BoardServiceGetGPIOResponse{High: high}, nil
}

// SetPWM sets a given pin of the underlying robot to the given duty cycle.
func (s *subtypeServer) SetPWM(ctx context.Context, req *pb.BoardServiceSetPWMRequest) (*pb.BoardServiceSetPWMResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.BoardServiceSetPWMResponse{}, b.SetPWM(ctx, req.Pin, req.DutyCyclePct)
}

// SetPWMFrequency sets a given pin of a board of the underlying robot to the given PWM frequency. 0 will use the board's default PWM
// frequency.
func (s *subtypeServer) SetPWMFrequency(
	ctx context.Context,
	req *pb.BoardServiceSetPWMFrequencyRequest,
) (*pb.BoardServiceSetPWMFrequencyResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.BoardServiceSetPWMFrequencyResponse{}, b.SetPWMFreq(ctx, req.Pin, uint(req.FrequencyHz))
}

// ReadAnalogReader reads off the current value of an analog reader of a board of the underlying robot.
func (s *subtypeServer) ReadAnalogReader(
	ctx context.Context,
	req *pb.BoardServiceReadAnalogReaderRequest,
) (*pb.BoardServiceReadAnalogReaderResponse, error) {
	b, err := s.getBoard(req.BoardName)
	if err != nil {
		return nil, err
	}

	theReader, ok := b.AnalogReaderByName(req.AnalogReaderName)
	if !ok {
		return nil, errors.Errorf("unknown analog reader: %s", req.AnalogReaderName)
	}

	val, err := theReader.Read(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.BoardServiceReadAnalogReaderResponse{Value: int32(val)}, nil
}

// GetDigitalInterruptValue returns the current value of the interrupt which is based on the type of interrupt.
func (s *subtypeServer) GetDigitalInterruptValue(
	ctx context.Context,
	req *pb.BoardServiceGetDigitalInterruptValueRequest,
) (*pb.BoardServiceGetDigitalInterruptValueResponse, error) {
	b, err := s.getBoard(req.BoardName)
	if err != nil {
		return nil, err
	}

	interrupt, ok := b.DigitalInterruptByName(req.DigitalInterruptName)
	if !ok {
		return nil, errors.Errorf("unknown digital interrupt: %s", req.DigitalInterruptName)
	}

	val, err := interrupt.Value(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.BoardServiceGetDigitalInterruptValueResponse{Value: val}, nil
}