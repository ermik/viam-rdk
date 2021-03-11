package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	"go.uber.org/multierr"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"
	"go.viam.com/robotcore/utils"
)

const (
	PanCenter  = 94
	TiltCenter = 113

	WheelCircumferenceMillis = math.Pi * 150
)

// ------

type Rover struct {
	theBoard board.Board

	fl, fr, bl, br board.Motor
	pan, tilt      board.Servo
}

func (r *Rover) Close(ctx context.Context) error {
	return r.Stop(ctx)
}

func (r *Rover) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
	if distanceMillis == 0 && block {
		return fmt.Errorf("cannot block unless you have a distance")
	}

	if distanceMillis != 0 && millisPerSec <= 0 {
		return fmt.Errorf("if distanceMillis is set, millisPerSec has to be positive")
	}

	var d board.Direction = board.DirForward
	if distanceMillis < 0 || millisPerSec < 0 {
		d = board.DirBackward
		distanceMillis = utils.AbsInt(distanceMillis)
		millisPerSec = math.Abs(millisPerSec)
	}

	var err error
	rotations := float64(distanceMillis) / WheelCircumferenceMillis

	rotationsPerSec := millisPerSec / WheelCircumferenceMillis
	rpm := 60 * rotationsPerSec

	err = multierr.Combine(
		r.fl.GoFor(d, rpm, rotations, false),
		r.fr.GoFor(d, rpm, rotations, false),
		r.bl.GoFor(d, rpm, rotations, false),
		r.br.GoFor(d, rpm, rotations, false),
	)

	if err != nil {
		return multierr.Combine(err, r.Stop(ctx))
	}

	if !block {
		return nil
	}

	return r.waitForMotorsToStop()
}

func (r *Rover) Spin(ctx context.Context, angleDeg float64, speed int, block bool) error {

	if speed < 120 {
		speed = 120
	}

	var a, b board.Direction = board.DirForward, board.DirBackward
	if angleDeg < 0 {
		a, b = board.DirBackward, board.DirForward
	}

	rotations := math.Abs(angleDeg / 5.0)

	rpm := float64(speed) // TODO(erh): fix me
	err := multierr.Combine(
		r.fl.GoFor(a, rpm, rotations, false),
		r.fr.GoFor(b, rpm, rotations, false),
		r.bl.GoFor(a, rpm, rotations, false),
		r.br.GoFor(b, rpm, rotations, false),
	)

	if err != nil {
		return multierr.Combine(err, r.Stop(ctx))
	}

	if !block {
		return nil
	}

	return r.waitForMotorsToStop()
}

func (r *Rover) waitForMotorsToStop() error {
	for {
		time.Sleep(10 * time.Millisecond)

		if r.fl.IsOn() ||
			r.fr.IsOn() ||
			r.bl.IsOn() ||
			r.br.IsOn() {
			continue
		}

		break
	}

	return nil
}

func (r *Rover) Stop(ctx context.Context) error {
	return multierr.Combine(
		r.fl.Off(),
		r.fr.Off(),
		r.bl.Off(),
		r.br.Off(),
	)
}

func (r *Rover) WidthMillis(ctx context.Context) (int, error) {
	return 600, nil
}

func (r *Rover) neckCenter() error {
	return r.neckPosition(PanCenter, TiltCenter)
}

func (r *Rover) neckOffset(left int) error {
	return r.neckPosition(uint8(PanCenter+(left*-30)), uint8(TiltCenter-20))
}

func (r *Rover) neckPosition(pan, tilt uint8) error {
	golog.Global.Debugf("neckPosition to %v %v", pan, tilt)
	return multierr.Combine(r.pan.Move(pan), r.tilt.Move(tilt))
}

func (r *Rover) Ready(theRobot api.Robot) error {
	golog.Global.Debugf("minirover2 Ready called")
	cam := theRobot.CameraByName("front")
	if cam == nil {
		return fmt.Errorf("no camera named front")
	}

	// doing this in a goroutine so i can see camera and servo data in web ui, but probably not right long term
	if false {
		go func() {
			for {
				time.Sleep(time.Second)
				var depthErr bool
				func() {
					img, release, err := cam.Next(context.Background())
					if err != nil {
						golog.Global.Debugf("error from camera %s", err)
						return
					}
					defer release()
					pc := rimage.ConvertToImageWithDepth(img)
					if pc.Depth == nil {
						golog.Global.Warnf("no depth data")
						depthErr = true
						return
					}
					err = pc.WriteTo(fmt.Sprintf("data/rover-centering-%d.both.gz", time.Now().Unix()))
					if err != nil {
						golog.Global.Debugf("error writing %s", err)
					}
				}()
				if depthErr {
					return
				}
			}
		}()
	}

	return nil
}

func NewRover(theBoard board.Board) (*Rover, error) {
	rover := &Rover{theBoard: theBoard}
	rover.fl = theBoard.Motor("fl-m")
	rover.fr = theBoard.Motor("fr-m")
	rover.bl = theBoard.Motor("bl-m")
	rover.br = theBoard.Motor("br-m")

	if rover.fl == nil || rover.fr == nil || rover.bl == nil || rover.br == nil {
		return nil, fmt.Errorf("missing a motor for minirover2")
	}

	rover.pan = theBoard.Servo("pan")
	rover.tilt = theBoard.Servo("tilt")

	if false {
		go func() {
			for {
				time.Sleep(1500 * time.Millisecond)
				err := rover.neckCenter()
				if err != nil {
					panic(err)
				}

				time.Sleep(1500 * time.Millisecond)

				err = rover.neckOffset(-1)
				if err != nil {
					panic(err)
				}

				time.Sleep(1500 * time.Millisecond)

				err = rover.neckOffset(1)
				if err != nil {
					panic(err)
				}

			}
		}()
	} else {
		err := rover.neckCenter()
		if err != nil {
			return nil, err
		}
	}

	golog.Global.Debug("rover ready")

	return rover, nil
}

func main() {
	err := realMain()
	if err != nil {
		log.Fatal(err)
	}
}

func realMain() error {
	flag.Parse()

	cfg, err := robot.ReadConfig("samples/minirover/config.json")
	if err != nil {
		return err
	}

	myRobot, err := robot.NewRobot(context.Background(), cfg)
	if err != nil {
		return err
	}
	defer myRobot.Close(context.Background())

	rover, err := NewRover(myRobot.BoardByName("local"))
	if err != nil {
		return err
	}

	myRobot.AddBase(rover, robot.Component{Name: "minirover"})

	err = rover.Ready(myRobot)
	if err != nil {
		return err
	}

	return web.RunWeb(myRobot)
}