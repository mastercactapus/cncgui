package spjs

type SerialPort struct {
	Name         string
	Friendly     string
	IsOpen       bool
	SerialNumber string
	VID          string `json:"UsbVid"`
	PID          string `json:"UsbPid"`
}
