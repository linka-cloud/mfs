// Copyright 2024 Linka Cloud  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mfs

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func Mount(path string, fs fs.FS) (MFS, error) {
	m := &mfs{}
	return m, m.Mount(path, fs)
}

type MFS interface {
	fs.ReadDirFS
	Mount(path string, fs fs.FS) error
}

var _ MFS = (*mfs)(nil)

type mfs struct {
	mapfs map[string]fs.FS
	mu    sync.RWMutex
}

func (m *mfs) Mount(path string, f fs.FS) error {
	path = filepath.Clean(path)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.mapfs == nil {
		m.mapfs = make(map[string]fs.FS)
	}
	if _, ok := m.mapfs[path]; ok {
		return fs.ErrExist
	}
	m.mapfs[path] = f
	return nil
}

func (m *mfs) Open(name string) (fs.File, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	name = filepath.Clean(name)
	open := func(fs fs.FS, n string) (fs.File, error) {
		f, err := fs.Open(n)
		if err != nil {
			return nil, err
		}
		return &file{File: f, path: name}, nil
	}
	if name == "." || name == "/" {
		return &fakeDir{path: name}, nil
	}
	for k, v := range m.mapfs {
		if name == k || name == k+"/" {
			return open(v, ".")
		}
		if len(name) > len(k) && name[:len(k)] == k && name[len(k)] == '/' {
			return open(v, name[len(k)+1:])
		}
	}
	return nil, fs.ErrNotExist
}

func (m *mfs) ReadDir(name string) ([]fs.DirEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	name = filepath.Clean(name)
	readDir := func(rfs fs.FS, n string) ([]fs.DirEntry, error) {
		if !strings.HasSuffix(name, "/") {
			name += "/"
		}
		ds, err := fs.ReadDir(rfs, n)
		if err != nil {
			return nil, err
		}
		var res []fs.DirEntry
		for _, d := range ds {
			res = append(res, &dirEntry{DirEntry: d, path: d.Name()})
		}
		return res, nil
	}
	if name == "/" || name == "." {
		var res []fs.DirEntry
		for k := range m.mapfs {
			res = append(res, &fakeDir{path: k})
		}
		return res, nil
	}
	for k, v := range m.mapfs {
		if name == k || name == k+"/" {
			return readDir(v, ".")
		}
		if len(name) > len(k) && name[:len(k)] == k && name[len(k)] == '/' {
			return readDir(v, name[len(k)+1:])
		}
	}
	return nil, fs.ErrNotExist
}

type file struct {
	fs.File
	path string
}

func (f *file) Stat() (fs.FileInfo, error) {
	i, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &fileInfo{
		FileInfo: i,
		path:     f.path,
	}, nil
}

type fileInfo struct {
	fs.FileInfo
	path string
}

func (f *fileInfo) Name() string {
	return f.path
}

type dirEntry struct {
	fs.DirEntry
	path string
}

func (d *dirEntry) Name() string {
	return d.path
}

var (
	_ fs.DirEntry = (*fakeDir)(nil)
	_ fs.FileInfo = (*fakeDir)(nil)
	_ fs.File     = (*fakeDir)(nil)
)

type fakeDir struct {
	path string
}

func (f *fakeDir) Stat() (fs.FileInfo, error) {
	return f.Info()
}

func (f *fakeDir) Read(_ []byte) (int, error) {
	return 0, &fs.PathError{Path: "/", Op: "read", Err: errors.New("is a directory")}
}

func (f *fakeDir) Close() error {
	return nil
}

func (f *fakeDir) Size() int64 {
	return 0
}

func (f *fakeDir) Mode() fs.FileMode {
	return fs.ModeDir
}

func (f *fakeDir) ModTime() time.Time {
	return time.Time{}
}

func (f *fakeDir) Sys() any {
	return nil
}

func (f *fakeDir) Name() string {
	return f.path
}

func (f *fakeDir) IsDir() bool {
	return true
}

func (f *fakeDir) Type() fs.FileMode {
	return fs.ModeDir
}

func (f *fakeDir) Info() (fs.FileInfo, error) {
	return f, nil
}
