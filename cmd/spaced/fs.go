package main

import (
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"gopkg.in/mattes/go-expand-tilde.v1"
)

type fswatchConfig struct {
	MonitorPath string `toml:"monitor_path"`
}

func (c fswatchConfig) GetService() (*fsnotify.Watcher, error) {
	if len(c.MonitorPath) == 0 {
		return nil, errors.New("monitor_path cannot be blank")
	}
	var err error
	if c.MonitorPath, err = tilde.Expand(c.MonitorPath); err != nil {
		return nil, err
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err = w.Add(c.MonitorPath); err != nil {
		return nil, err
	}
	return w, nil
}
