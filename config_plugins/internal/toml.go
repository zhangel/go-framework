package internal

import (
	"fmt"

	"github.com/pelletier/go-toml"
)

type TomlReader struct {
	path string
}

func NewTomlReader(path string) (FileReader, error) {
	return &TomlReader{path}, nil
}

func (s *TomlReader) Read() (map[string]string, error) {
	config := make(map[string]string)

	tomlFile, err := toml.LoadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read config toml file failed, err = %v", err)
	}

	Walk("", tomlFile.ToMap(), &config)

	return config, nil
}

func (s *TomlReader) Close() error {
	return nil
}
