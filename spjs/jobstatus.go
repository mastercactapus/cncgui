package spjs

type JobStatus struct {
	Name   string
	Valid  bool
	Active bool

	Read         int
	ReadComplete bool
	Sent         int
	Completed    int

	Err error
}
