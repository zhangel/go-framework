package config_plugins

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/zhangel/go-framework/uri"

	"github.com/fsnotify/fsnotify"

	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/config/watcher"
	"github.com/zhangel/go-framework/config_plugins/internal"
	"github.com/zhangel/go-framework/declare"
)

const (
	filePlugin = "file"
	flagPath   = "path"
	flagType   = "type"
	flagReload = "reload"
)

var readerFactory = map[string]func(path string) (internal.FileReader, error){
	"ini":   internal.NewIniReader,
	"yaml":  internal.NewYamlReader,
	"json":  internal.NewJsonReader,
	"toml":  internal.NewTomlReader,
	"hocon": internal.NewHoconReader,
	"conf":  internal.NewHoconReader,
}

type FileConfigSource struct {
	path      string
	reader    internal.FileReader
	notifier  *watcher.Notifier
	watchDone chan struct{}
	reload    bool

	once sync.Once
}

func init() {
	declare.Plugin(config.Plugin, declare.PluginInfo{Name: filePlugin, Creator: func(cfg config.Config) (*FileConfigSource, error) {
		return NewFileConfigSource(cfg.String(flagPath), cfg.String(flagType), cfg.Bool(flagReload))
	}},
		declare.Flag{Name: flagPath, DefaultValue: "./config.yaml", Description: "Config file path.", UriField: uri.UriFieldPath},
		declare.Flag{Name: flagType, DefaultValue: "auto", Description: "Config file type. optionals: auto, yaml, json, toml, ini, hocon."},
		declare.Flag{Name: flagReload, DefaultValue: false, Description: "Auto reload while config file changed."},
	)
}

func NewFileConfigSource(path, typ string, autoReload bool) (*FileConfigSource, error) {
	if typ == "" {
		typ = "auto"
	}

	if reader, err := createReader(path, typ); err != nil {
		return nil, err
	} else {
		return &FileConfigSource{
			path:      path,
			reader:    reader,
			notifier:  watcher.NewNotifier(),
			watchDone: make(chan struct{}),
			reload:    autoReload,
		}, nil
	}
}

func (s *FileConfigSource) Sync() (map[string]string, error) {
	return s.reader.Read()
}

func (s *FileConfigSource) Watch(watcher watcher.SourceWatcher) (cancel func()) {
	if !s.reload {
		return
	}

	ctx, cancelFunc := context.WithCancel(context.Background())
	cancelWatch := s.notifier.Watch(watcher)

	cancel = func() {
		cancelFunc()
		cancelWatch()
	}

	s.once.Do(func() {
		go func() {
			w, err := fsnotify.NewWatcher()
			if err != nil {
				log.Printf("Register fsnotify watcher failed, err = %v\n", err)
				return
			}

			defer func() {
				_ = w.Close()
			}()

			err = w.Add(s.path)
			if err != nil {
				log.Printf("Add config path to fsnotify watcher failed, path = %s, err = %v\n", s.path, err)
				return
			}

			go func() {
				if fileProvider, ok := s.reader.(internal.AdditionalFileProvider); !ok {
					return
				} else {
					for {
						select {
						case v := <-fileProvider.Watch():
							if v.Canceled {
								return
							}

							if err := w.Add(v.FilePath); err != nil {
								log.Printf("Add config path to fsnotify watcher failed, path = %s, err = %v\n", v.FilePath, err)
							}
						}
					}
				}
			}()

			for {
				select {
				case event, ok := <-w.Events:
					if !ok {
						log.Printf("Bump from watcher.Events failed, err = %v\n", err)
						return
					}

					if event.Op&fsnotify.Rename == fsnotify.Rename {
						time.Sleep(time.Second)
						err = w.Add(event.Name)
						if err != nil {
							log.Printf("Add config path to fsnotify watcher failed, path = %s, err = %v\n", s.path, err)
							return
						}
					}

					go func() {
						if (event.Op&fsnotify.Write == fsnotify.Write) || (event.Op&fsnotify.Rename == fsnotify.Rename) {
							if val, err := s.Sync(); err == nil {
								s.notifier.OnSync(val)
							}
						}
					}()
				case err, ok := <-w.Errors:
					if !ok {
						log.Printf("Bump from watcher.Errors failed, err = %v\n", err)
						return
					}
				case <-s.watchDone:
					return
				case <-ctx.Done():
					return
				}
			}
		}()
	})

	return cancel
}

func (s *FileConfigSource) AppendPrefix(_ []string) error {
	return nil
}

func (s *FileConfigSource) Close() error {
	close(s.watchDone)
	if s.reader != nil {
		return s.reader.Close()
	}
	return nil
}

func createReader(filePath, fileType string) (internal.FileReader, error) {
	if fileType == "auto" {
		fileType = filepath.Ext(filePath)
		if fileType != "" && fileType[0] == '.' {
			fileType = fileType[1:]
		}
	}

	if factory, ok := readerFactory[fileType]; ok {
		return factory(filePath)
	} else {
		return nil, fmt.Errorf("unknown file type, type = %s", fileType)
	}
}

func (s *FileConfigSource) IgnoreNamespace() {}
