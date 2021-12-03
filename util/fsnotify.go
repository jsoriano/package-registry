// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type Event = fsnotify.Event

// DirectoryWatcher watches changes in a directory recursively.
type DirectoryWatcher struct {
	lock    sync.Mutex
	watcher *fsnotify.Watcher
	paths   map[string]struct{}
	done    chan struct{}

	Events chan Event
	Errors chan error
}

// NewDirectoryWatcher creates a new directory watcher.
func NewDirectoryWatcher() (*DirectoryWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := DirectoryWatcher{
		watcher: watcher,
		paths:   make(map[string]struct{}),
		done:    make(chan struct{}),
		Events:  make(chan Event),
		Errors:  make(chan error),
	}
	go w.loop()

	return &w, nil
}

func (w *DirectoryWatcher) loop() {
	// Errors and Events need to be read on different goroutines.
	go func() {
		for {
			select {
			case err := <-w.watcher.Errors:
				w.Errors <- err
			case <-w.done:
				return
			}
		}
	}()

	for {
		select {
		case e := <-w.watcher.Events:
			switch e.Op {
			case fsnotify.Create:
				go w.handleCreate(e.Name)
			case fsnotify.Remove:
				go w.handleRemove(e.Name)
			case fsnotify.Rename:
				go w.handleRename(e.Name)
			}
			w.Events <- e
		case <-w.done:
			return
		}
	}
}

func (w *DirectoryWatcher) handleCreate(name string) {
	info, _ := os.Stat(name)
	if info.IsDir() {
		w.addRecursive(name)
	}
}

func (w *DirectoryWatcher) handleRemove(name string) {
	info, _ := os.Stat(name)
	if info.IsDir() {
		w.removeRecursive(name)
	}
}

func (w *DirectoryWatcher) handleRename(name string) {
	// ??
}

func (w *DirectoryWatcher) Add(path string) error {
	return w.addRecursive(path)
}

func (w *DirectoryWatcher) addRecursive(path string) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	return filepath.WalkDir(path, func(path string, info os.DirEntry, err error) error {
		if !info.IsDir() {
			return nil
		}

		if _, found := w.paths[path]; found {
			// Already watched.
			return nil
		}

		return w.watcher.Add(path)
	})
}

func (w *DirectoryWatcher) Remove(path string) error {
	return w.removeRecursive(path)
}

func (w *DirectoryWatcher) removeRecursive(path string) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	for p := range w.paths {
		if _, err := filepath.Rel(path, p); err != nil {
			continue
		}

		if err := w.watcher.Remove(p); err != nil {
			return err
		}

		delete(w.paths, p)
	}

	return nil
}

func (w *DirectoryWatcher) Close() error {
	close(w.done)
	return w.watcher.Close()
}
