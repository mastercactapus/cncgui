package spjs

type Driver interface {
	Name() string

	BufferAlgorithm() string
	BaudRate() int

	SetPort(*Port)

	HandleData(string) error
}
