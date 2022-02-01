package internal

type FileReader interface {
	Read() (map[string]string, error)
	Close() error
}
