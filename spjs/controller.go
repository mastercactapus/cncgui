package spjs

type Controller interface {
	Name() string
	Connected() bool

	CommandHome(wait bool) error
	CommandEStop() error
	CommandJog(axis rune, mm float64, wait bool) error

	SetWPos(axis rune, mm float64) error

	LastStatus() ControllerStatus
	Status() <-chan ControllerStatus
}

type ControllerStatus interface {
	MachinePosition() Position
	WorkPosition() Position

	StatusText() string

	IsReady() bool
	IsAlarm() bool
}

type Position struct{ X, Y, Z float64 }
