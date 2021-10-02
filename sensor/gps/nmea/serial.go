package nmea

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/adrianmo/go-nmea"
	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/gps"
	"go.viam.com/core/serial"
)

func init() {
	registry.RegisterSensor(gps.Type, "nmea-serial", registry.Sensor{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
		return newSerialNMEAGPS(config, logger)
	}})
}

type serialNMEAGPS struct {
	mu     sync.RWMutex
	dev    io.ReadWriteCloser
	logger golog.Logger

	lastLocation *geo.Point

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

const pathAttrName = "path"

func newSerialNMEAGPS(config config.Component, logger golog.Logger) (*serialNMEAGPS, error) {
	serialPath := config.Attributes.String(pathAttrName)
	if serialPath == "" {
		return nil, fmt.Errorf("serialNMEAGPS expected non-empty string for %q", pathAttrName)
	}
	dev, err := serial.Open(serialPath)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	g := &serialNMEAGPS{dev: dev, cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	g.Start()

	return g, nil
}

func (g *serialNMEAGPS) Start() {
	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer g.activeBackgroundWorkers.Done()
		r := bufio.NewReader(g.dev)
		for {
			select {
			case <-g.cancelCtx.Done():
				return
			default:
			}

			line, err := r.ReadString('\n')
			if err != nil {
				g.logger.Fatalf("can't read gps serial %s", err)
			}

			s, err := nmea.Parse(line)
			if err != nil {
				g.logger.Debugf("can't parse nmea %s : %s", line, err)
				continue
			}

			gll, ok := s.(nmea.GLL)
			if ok {
				now := toPoint(gll)
				g.mu.Lock()
				g.lastLocation = now
				g.mu.Unlock()
			}
		}
	})
}

func (g *serialNMEAGPS) Readings(ctx context.Context) ([]interface{}, error) {
	lat, long, err := g.Location(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{lat, long}, nil
}

func (g *serialNMEAGPS) Location(ctx context.Context) (lat float64, long float64, err error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.lastLocation.Lat(), g.lastLocation.Lng(), nil
}

func (g *serialNMEAGPS) Close() error {
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()
	return g.dev.Close()
}

// Desc returns that this is a GPS.
func (g *serialNMEAGPS) Desc() sensor.Description {
	return sensor.Description{gps.Type, ""}
}

// toPoint converts a nmea.GLL to a geo.Point
func toPoint(a nmea.GLL) *geo.Point {
	return geo.NewPoint(a.Latitude, a.Longitude)
}