package spjs

// NewVIDPIDMatcher returns a SerialPortMatcher that returns true if the usb vendor and product IDs match.
func NewVIDPIDMatcher(vid, pid string) SerialPortMatcher {
	return func(sp SerialPort) bool {
		return sp.VID == vid && sp.PID == pid
	}
}
