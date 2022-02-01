package internal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type JsonReader struct {
	path string
}

func NewJsonReader(path string) (FileReader, error) {
	return &JsonReader{path}, nil
}

func (s *JsonReader) Read() (map[string]string, error) {
	config := map[string]string{}

	source, err := ioutil.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read config json file failed, err = %v", err)
	}

	content := map[string]interface{}{}
	err = json.Unmarshal(source, &content)
	if err != nil {
		return nil, fmt.Errorf("read config json file failed, err = %v", err)
	}

	Walk("", content, &config)
	return config, nil
}

func (s *JsonReader) Close() error {
	return nil
}
