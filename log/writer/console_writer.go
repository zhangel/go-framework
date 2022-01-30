package writer

import (
	"fmt"
	"os"
)

type ConsoleWriter struct {
	f *os.File
}

func NewConsoleWriter(stdout bool) (*ConsoleWriter, error) {
	if stdout {
		return &ConsoleWriter{f: os.Stdout}, nil
	} else {
		return &ConsoleWriter{f: os.Stdout}, nil
	}
}

func (c *ConsoleWriter) Write(bytes []byte) error {
	fmt.Fprintln(c.f, string(bytes))
	return nil
}

func (c *ConsoleWriter) Close() error {
	return nil
}
