package spjs

import (
	"fmt"
	"strings"
)

type ArduinoPendant struct {
	ctrl Controller
}

var _ Driver = &ArduinoPendant{}

// NewArduinoPendant will create a new pendant driver that will relay commands to the provided controller.
func NewArduinoPendant(ctrl Controller) *ArduinoPendant { return &ArduinoPendant{ctrl: ctrl} }

// Name always returns `ArduinoPendant`.
func (p *ArduinoPendant) Name() string { return "ArduinoPendant" }

// BufferAlgorithm always returns `default` (no buffer).
func (p *ArduinoPendant) BufferAlgorithm() string { return "default" }

// BaudRate always returns `115200`.
func (p *ArduinoPendant) BaudRate() int { return 115200 }

// SetPort has no effect as the pendant is read-only.
func (p *ArduinoPendant) SetPort(*Port) {}

// HandleData will process requests from the pendant and pass them to the controller.
func (p *ArduinoPendant) HandleData(data string) error {
	if strings.TrimSpace(data) == "STOP" {
		return p.ctrl.CommandEStop()
	}

	if !strings.HasPrefix(data, "STEP") {
		return nil
	}

	var axisIndex, mult, step int
	_, err := fmt.Sscanf(data, "STEP:%d,%d,%d", &axisIndex, &mult, &step)
	if err != nil {
		return err
	}

	var axis rune
	switch axisIndex {
	case 1:
		axis = 'X'
	case 2:
		axis = 'Y'
	case 3:
		axis = 'Z'

		// invert Z
		step = -step
	default:
		return nil
	}

	return p.ctrl.CommandJog(axis, float64(step)*float64(mult)/100, false)
}
