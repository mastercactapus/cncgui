package spjs

import (
	"context"
	"errors"
)

var ErrUnsupportedByDriver = errors.New("not supported by driver")

type Driver interface {
	Name() string

	BufferAlgorithm() string
	BaudRate() int

	HandleData(context.Context, string) error
}

type FeedHolder interface{ FeedHold() string }
type CycleStarter interface{ CycleStart() string }
type Resetter interface{ Reset() string }

type GCodeWrapper interface {
	WrapGCode(commands []string) string
}
type Homeable interface{ Home() string }
type EStopable interface{ EStop() string }
type Joggable interface {
	Jog(axis rune, mm float64) string
}
type WPosable interface {
	WPos(axis rune, mm float64) string
}
type Statusable interface {
	Status() <-chan ControllerStatus
}
