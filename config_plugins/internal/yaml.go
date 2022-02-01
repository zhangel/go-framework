package internal

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type YamlReader struct {
	path string
}

func NewYamlReader(path string) (FileReader, error) {
	return &YamlReader{path}, nil
}

func (s *YamlReader) Read() (map[string]string, error) {
	config := map[string]string{}

	source, err := ioutil.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read config yaml file failed, err = %v", err)
	}

	content := map[string]interface{}{}
	err = yaml.Unmarshal(source, &content)
	if err != nil {
		return nil, fmt.Errorf("read config yaml file failed, err = %v", err)
	}

	Walk("", content, &config)
	return config, nil
}

func (s *YamlReader) Close() error {
	return nil
}
