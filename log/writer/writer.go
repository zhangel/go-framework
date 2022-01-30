package writer

type Writer interface {
	Write([]byte) error
	Close() error
}
