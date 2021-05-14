package base_test

import (
	"context"
	"math"
	"testing"

	"github.com/go-errors/errors"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/core/base"
	"go.viam.com/core/testutils/inject"
)

func TestAugmentReduce(t *testing.T) {
	logger := golog.NewTestLogger(t)
	dev := &inject.Base{}
	dev.WidthMillisFunc = func(ctx context.Context) (int, error) {
		return 600, nil
	}
	test.That(t, base.AugmentWithCompass(dev, nil, logger), test.ShouldEqual, dev)

	comp := &inject.Compass{}
	aug := base.AugmentWithCompass(dev, comp, logger)
	test.That(t, aug, test.ShouldNotEqual, dev)
	var baseDev *base.Base = nil
	test.That(t, aug, test.ShouldImplement, baseDev)

	test.That(t, base.Reduce(aug), test.ShouldEqual, dev)
}

func TestDeviceWithCompass(t *testing.T) {
	logger := golog.NewTestLogger(t)
	dev := &inject.Base{}
	dev.WidthMillisFunc = func(ctx context.Context) (int, error) {
		return 600, nil
	}
	comp := &inject.Compass{}
	aug := base.AugmentWithCompass(dev, comp, logger)

	t.Run("perfect base", func(t *testing.T) {
		i := 0
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
			i++
			return 2.4, nil
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			if i == 0 {
				return 0, nil
			}
			return 10, nil
		}
		ang, _, err := base.DoMove(context.Background(), base.Move{AngleDeg: 10}, aug)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ang, test.ShouldEqual, 10)
	})

	t.Run("off by under third", func(t *testing.T) {
		i := 0
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
			i++
			return 2.4, nil
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			if i == 0 {
				return 0, nil
			}
			return 10 * (float64(i) / 3), nil
		}
		ang, _, err := base.DoMove(context.Background(), base.Move{AngleDeg: 10}, aug)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ang, test.ShouldEqual, 10)
	})

	t.Run("off by over third", func(t *testing.T) {
		i := 0
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
			i++
			return 2.4, nil
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			if i == 0 {
				return 0, nil
			}
			return 10 + 10*(float64(i)/3), nil
		}
		ang, _, err := base.DoMove(context.Background(), base.Move{AngleDeg: 10}, aug)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ang, test.ShouldEqual, 10)
	})

	t.Run("error getting heading", func(t *testing.T) {
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
			return angleDeg, nil
		}
		err1 := errors.New("oh no")
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			return math.NaN(), err1
		}
		ang, _, err := base.DoMove(context.Background(), base.Move{AngleDeg: 10}, aug)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
		test.That(t, ang, test.ShouldEqual, 0)
	})

	t.Run("error spinning", func(t *testing.T) {
		err1 := errors.New("oh no")
		dev.SpinFunc = func(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) (float64, error) {
			return 2.4, err1
		}
		comp.HeadingFunc = func(ctx context.Context) (float64, error) {
			return 0, nil
		}
		ang, _, err := base.DoMove(context.Background(), base.Move{AngleDeg: 10}, aug)
		test.That(t, errors.Is(err, err1), test.ShouldBeTrue)
		test.That(t, ang, test.ShouldEqual, 2.4)
	})
}