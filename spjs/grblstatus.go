package spjs

import (
	"fmt"
	"strings"
)

type GRBLStatus struct {
	Status          string
	MPos, WPos, WCO Position

	Feed     float64
	Spindle  float64
	Pins     GRBLPinStatus
	Override struct {
		Feed, Rapid, Spindle float64
	}
	Accesory GRBLACCStatus
}

var _ ControllerStatus = GRBLStatus{}

type GRBLPinStatus struct{ X, Y, Z, P, D, H, R, S bool }
type GRBLACCStatus struct {
	SpindleEnabled bool
	SpindleCCW     bool
	Flood          bool
	Mist           bool
}

func (stat GRBLStatus) IsAlarm() bool             { return strings.HasPrefix(stat.Status, "ALARM") }
func (stat GRBLStatus) IsReady() bool             { return stat.Status == "Idle" }
func (stat GRBLStatus) MachinePosition() Position { return stat.MPos }
func (stat GRBLStatus) WorkPosition() Position    { return stat.WPos }
func (stat GRBLStatus) StatusText() string        { return stat.Status }

func (stat *GRBLStatus) Parse(data string) error {

	data = strings.TrimSpace(data)
	data = strings.TrimPrefix(data, "<")
	data = strings.TrimSuffix(data, ">")
	parts := strings.Split(data, "|")
	stat.Status = parts[0]
	stat.Pins = GRBLPinStatus{}
	var useMPos bool

	for _, part := range parts[1:] {
		p := strings.SplitN(part, ":", 2)

		var err error
		switch p[0] {
		case "MPos":
			useMPos = true
			_, err = fmt.Sscanf(p[1], "%f,%f,%f", &stat.MPos.X, &stat.MPos.Y, &stat.MPos.Z)
			stat.WPos.X = stat.MPos.X - stat.WCO.X
			stat.WPos.Y = stat.MPos.Y - stat.WCO.Y
			stat.WPos.Z = stat.MPos.Z - stat.WCO.Z
		case "WPos":
			_, err = fmt.Sscanf(p[1], "%f,%f,%f", &stat.WPos.X, &stat.WPos.Y, &stat.WPos.Z)
			stat.MPos.X = stat.WPos.X + stat.WCO.X
			stat.MPos.Y = stat.WPos.Y + stat.WCO.Y
			stat.MPos.Z = stat.WPos.Z + stat.WCO.Z
		case "WCO":
			_, err = fmt.Sscanf(p[1], "%f,%f,%f", &stat.WCO.X, &stat.WCO.Y, &stat.WCO.Z)
			if useMPos {
				stat.WPos.X = stat.MPos.X - stat.WCO.X
				stat.WPos.Y = stat.MPos.Y - stat.WCO.Y
				stat.WPos.Z = stat.MPos.Z - stat.WCO.Z
			} else {
				stat.MPos.X = stat.WPos.X + stat.WCO.X
				stat.MPos.Y = stat.WPos.Y + stat.WCO.Y
				stat.MPos.Z = stat.WPos.Z + stat.WCO.Z
			}
		case "F":
			_, err = fmt.Sscanf(p[1], "%f", &stat.Feed)
		case "FS":
			_, err = fmt.Sscanf(p[1], "%f,%f", &stat.Feed, &stat.Spindle)
		case "Pn":
			stat.Pins.parse(p[1])
		case "Ov":
			_, err = fmt.Sscanf(p[1], "%f,%f,%f", &stat.Override.Feed, &stat.Override.Rapid, &stat.Override.Spindle)
		case "A":
			stat.Accesory.SpindleEnabled = strings.ContainsAny(p[1], "SC")
			stat.Accesory.SpindleCCW = strings.ContainsRune(p[1], 'C')
			stat.Accesory.Flood = strings.ContainsRune(p[1], 'F')
			stat.Accesory.Mist = strings.ContainsRune(p[1], 'M')
		}
		if err != nil {
			return fmt.Errorf("parse %s '%s': %w", p[0], p[1], err)
		}
	}

	return nil
}
func (pins *GRBLPinStatus) parse(s string) {
	pins.X = strings.ContainsRune(s, 'X')
	pins.Y = strings.ContainsRune(s, 'Y')
	pins.Z = strings.ContainsRune(s, 'Z')
	pins.P = strings.ContainsRune(s, 'P')
	pins.D = strings.ContainsRune(s, 'D')
	pins.H = strings.ContainsRune(s, 'H')
	pins.R = strings.ContainsRune(s, 'R')
	pins.S = strings.ContainsRune(s, 'S')
}
