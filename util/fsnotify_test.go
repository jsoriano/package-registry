// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package util

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileAdded(t *testing.T) {
	w, err := NewDirectoryWatcher()
	require.NoError(t, err)
	defer func() {
		require.NoError(t, w.Close())
	}()

	tmpdir := t.TempDir()
	err = w.Add(tmpdir)
	require.NoError(t, err)

	filePath := filepath.Join(tmpdir, "file")
	err = ioutil.WriteFile(filePath, []byte{}, 0644)
	require.NoError(t, err)

	e := expectEvent(t, w)
	assert.Equal(t, fsnotify.Create, e.Op)
	assert.Equal(t, filePath, e.Name)
}

func TestFileAddedToSubdirectory(t *testing.T) {
	w, err := NewDirectoryWatcher()
	require.NoError(t, err)
	defer func() {
		require.NoError(t, w.Close())
	}()

	tmpdir := t.TempDir()
	err = w.Add(tmpdir)
	require.NoError(t, err)

	subdir := filepath.Join(tmpdir, "dir")
	os.Mkdir(subdir, 0755)
	e := expectEvent(t, w)
	assert.Equal(t, fsnotify.Create, e.Op)
	assert.Equal(t, subdir, e.Name)

	// TODO: Race condition here

	filePath := filepath.Join(subdir, "file")
	err = ioutil.WriteFile(filePath, []byte{}, 0644)
	require.NoError(t, err)

	e = expectEvent(t, w)
	assert.Equal(t, fsnotify.Create, e.Op)
	assert.Equal(t, filePath, e.Name)
}

func TestStopMonitoringDirectory(t *testing.T) {
	// TODO
}

func TestDirectoryMoved(t *testing.T) {
	// TODO
}

func expectEvent(t *testing.T, w *DirectoryWatcher) Event {
	t.Helper()

	select {
	case e := <-w.Events:
		return e
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	return Event{}
}
