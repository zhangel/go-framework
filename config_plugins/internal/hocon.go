package internal

import (
	"fmt"
	"sync/atomic"

	"github.com/zhangel/go-framework/config_plugins/internal/3rdparty/hocon"
	"github.com/zhangel/go-framework/internal"
)

type HoconReader struct {
	path           string
	fileChangeChan chan AdditionalFileResp
	closed         uint32
}

func NewHoconReader(path string) (FileReader, error) {
	return &HoconReader{path: path, fileChangeChan: make(chan AdditionalFileResp)}, nil
}

func (s *HoconReader) Read() (map[string]string, error) {
	hoconConfig, err := hocon.ParseResource(s.path)
	if err != nil {
		return nil, fmt.Errorf("parse hocon config file failed, err = %v", err)
	}

	config := make(map[string]string)
	hoconConfig.Walk("", func(path string, val hocon.Value) bool {
		switch val.Type() {
		case hocon.StringType, hocon.BooleanType:
			config[path] = val.String()
		case hocon.NumberType:
			switch v := val.(type) {
			case hocon.Int:
				config[path] = internal.Stringify(int(v))
			case hocon.Float32:
				config[path] = internal.Stringify(float32(v))
			case hocon.Float64:
				config[path] = internal.Stringify(float64(v))
			default:
				config[path] = val.String()
			}
		}
		return true
	})

	for _, include := range hoconConfig.Includes() {
		s.fileChangeChan <- AdditionalFileResp{FilePath: include}
	}

	return config, nil
}

func (s *HoconReader) Close() error {
	atomic.StoreUint32(&s.closed, 1)
	s.fileChangeChan <- AdditionalFileResp{Canceled: true}
	close(s.fileChangeChan)
	return nil
}

func (s *HoconReader) Watch() <-chan AdditionalFileResp {
	return s.fileChangeChan
}
