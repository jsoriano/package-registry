// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package packages

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// PackageFile is the interface that files in the file system need to implement.
// Seeker interface is needed for http helpers to serve files.
type PackageFile interface {
	fs.File
	io.ReadSeeker
}

type PackageFileSystem interface {
	Open(name string) (fs.File, error)
	Close() error
}

// ExtractedPackageFileSystem provides utils to access files in an extracted package.
type ExtractedPackageFileSystem struct {
	fs.FS
}

func NewExtractedPackageFileSystem(p *Package) (*ExtractedPackageFileSystem, error) {
	return &ExtractedPackageFileSystem{
		FS: &absolutePathFileSystem{os.DirFS(p.BasePath)},
	}, nil
}

func (fs *ExtractedPackageFileSystem) Close() error { return nil }

// ZipPackageFileSystem provides utils to access files in a zipped package.
type ZipPackageFileSystem struct {
	fs.FS

	reader *zip.ReadCloser
}

func NewZipPackageFileSystem(p *Package) (*ZipPackageFileSystem, error) {
	reader, err := zip.OpenReader(p.BasePath)
	if err != nil {
		return nil, err
	}
	var root string
	found := false
	for _, f := range reader.File {
		name := filepath.Clean(f.Name)
		parts := strings.Split(name, string(filepath.Separator))
		if len(parts) == 2 && parts[1] == "manifest.yml" {
			root = parts[0]
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("failed to determine root directory in package (path: %s)", p.BasePath)
	}

	sub, err := fs.Sub(reader, root)
	if err != nil {
		return nil, fmt.Errorf("failed to open root directory in package (path: %s)", p.BasePath)
	}
	return &ZipPackageFileSystem{
		FS:     &absolutePathFileSystem{sub},
		reader: reader,
	}, nil
}

func (fs *ZipPackageFileSystem) Open(name string) (fs.File, error) {
	f, err := fs.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return &zipFileSeeker{
		File: f,
		fs:   fs.FS,
		path: name,
	}, nil
}

func (fs *ZipPackageFileSystem) Close() error {
	return fs.reader.Close()
}

// zipFileSeeker implements the seeker interface for zip files.
type zipFileSeeker struct {
	fs.File

	fs   fs.FS
	path string
}

// Ensure that zipFileSeeker implements PackageFile interface
var _ PackageFile = &zipFileSeeker{}

// Seek implements the seeker interface for zip files. This is inefficient, it shouldn't
// be frequently used.
func (f *zipFileSeeker) Seek(offset int64, whence int) (n int64, err error) {
	switch whence {
	case io.SeekStart:
		f.File.Close()
		f.File, err = f.fs.Open(f.path)
		if err != nil {
			return -1, err
		}
		if offset > 0 {
			r := io.LimitReader(f.File, offset)
			n, err = io.Copy(ioutil.Discard, r)
			if err != nil {
				return -1, err
			}
			offset = int64(n)
		}
		return offset, nil
	case io.SeekEnd:
		if offset != 0 {
			return -1, fmt.Errorf("unsupported offset")
		}
		info, err := f.File.Stat()
		if err != nil {
			return -1, err
		}
		_, err = io.Copy(ioutil.Discard, f.File)
		if err != nil {
			return -1, err
		}
		return info.Size(), nil
	default:
		return -1, fmt.Errorf("unsupported whence")
	}
}

// VirtualPackageFileSystem provide utils for package objects that don't correspond to
// any real package in any backend. Used mainly for testing purpouses.
type VirtualPackageFileSystem struct{}

func NewVirtualPackageFileSystem() (*VirtualPackageFileSystem, error) {
	return &VirtualPackageFileSystem{}, nil
}

func (fs *VirtualPackageFileSystem) Stat(name string) (os.FileInfo, error) {
	return nil, os.ErrNotExist
}

func (fs *VirtualPackageFileSystem) Open(name string) (fs.File, error) {
	return nil, os.ErrNotExist
}

func (fs *VirtualPackageFileSystem) Close() error { return nil }

type absolutePathFileSystem struct {
	fs.FS
}

func (fs *absolutePathFileSystem) Open(name string) (fs.File, error) {
	if strings.HasPrefix(name, "./") {
		name = strings.TrimPrefix(name, "./")
	}
	if strings.HasPrefix(name, "/") {
		name = strings.TrimPrefix(name, "/")
	}
	return fs.FS.Open(name)
}
