package vsphere

import (
	"os"
	"io"
	"archive/tar"
	"path"
)

type OvaFile struct {
	path string
}

func (t *OvaFile) Open(name string) (io.ReadCloser, int64, error) {
	f, err := os.Open(t.path)
	if err != nil {
		return nil, 0, err
	}

	r := tar.NewReader(f)

	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, err
		}

		matched, err := path.Match(name, path.Base(h.Name))
		if err != nil {
			return nil, 0, err
		}

		if matched {
			return &OvaFileEntry{r, f}, h.Size, nil
		}
	}

	_ = f.Close()

	return nil, 0, os.ErrNotExist
}

type OvaFileEntry struct {
	io.Reader
	f *os.File
}

func (t *OvaFileEntry) Close() error {
	return t.f.Close()
}
