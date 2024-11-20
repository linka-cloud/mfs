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
	"io"
	"io/fs"
	"strings"
	"testing"

	"github.com/psanford/memfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var data = map[string][]byte{
	"foo":    []byte("bar"),
	"baz":    []byte("qux"),
	"quux":   []byte("corge"),
	"grault": []byte("garply"),
}

func Test(t *testing.T) {

	tests := []struct {
		prefix string
	}{
		{""},
		{"."},
		{"/"},
		{"./"},
	}

	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			m1 := memfs.New()
			m2 := memfs.New()
			require.NoError(t, m1.MkdirAll("1", 0755))
			require.NoError(t, m1.MkdirAll("2", 0755))

			for k, v := range data {
				require.NoError(t, m1.WriteFile(k, v, 0666))
				require.NoError(t, m1.WriteFile("1/"+k, v, 0666))
				require.NoError(t, m2.WriteFile("2"+k, v, 0666))
			}
			var (
				mfs MFS
				err error
			)
			p := tt.prefix
			if p == "." {
				p = ""
			}
			mfs, err = Mount(p+"m1", m1)
			require.NoError(t, err)
			require.NoError(t, mfs.Mount(p+"m2", m2))

			t.Run("open root", func(t *testing.T) {
				f, err := mfs.Open(tt.prefix)
				require.NoError(t, err)
				defer f.Close()
				s, err := f.Stat()
				require.NoError(t, err)
				assert.True(t, s.IsDir())
				_, err = io.ReadAll(f)
				require.Error(t, err)
				assert.Equal(t, "read /: is a directory", err.Error())
			})

			t.Run("open non-existent", func(t *testing.T) {
				_, err = mfs.Open("foo")
				assert.ErrorIs(t, err, fs.ErrNotExist)
			})

			t.Run("open", func(t *testing.T) {
				f, err := mfs.Open(p + "m1/1/foo")
				require.NoError(t, err)
				require.NotNil(t, f)
				defer f.Close()
				s, err := f.Stat()
				require.NoError(t, err)
				p := ""
				if strings.HasPrefix(tt.prefix, "/") {
					p = "/"
				}
				assert.Equal(t, p+"m1/1/foo", s.Name())
				b, err := io.ReadAll(f)
				require.NoError(t, err)
				assert.Equal(t, data["foo"], b)
			})

			t.Run("read root dir", func(t *testing.T) {
				d, err := mfs.ReadDir(tt.prefix)
				require.NoError(t, err)
				require.Len(t, d, 2)
				for _, v := range d {
					assert.True(t, v.IsDir())
					fn := assert.False
					if strings.HasPrefix(tt.prefix, "/") {
						fn = assert.True
					}
					fn(t, strings.HasPrefix(v.Name(), "/"))
				}
			})

			t.Run("read dir", func(t *testing.T) {
				d, err := mfs.ReadDir(p + "m1/1")
				require.NoError(t, err)
				require.Len(t, d, 4)
			})
		})
	}
}
