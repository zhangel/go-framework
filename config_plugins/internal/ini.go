package internal

import (
	"fmt"

	"github.com/go-ini/ini"
)

type IniReader struct {
	path string
}

func NewIniReader(path string) (FileReader, error) {
	return &IniReader{path}, nil
}

func (s *IniReader) Read() (map[string]string, error) {
	config := make(map[string]string)

	iniFile, err := ini.Load(s.path)
	if err != nil {
		return nil, fmt.Errorf("read config ini file failed, err = %v", err)
	}

	sections := iniFile.Sections()
	for _, section := range sections {
		prefix := section.Name() + "."
		if section.Name() == ini.DefaultSection {
			prefix = ""
		}

		for _, entry := range section.Keys() {
			config[prefix+entry.Name()] = entry.String()
		}
	}

	return config, nil
}

func (s *IniReader) Close() error {
	return nil
}
