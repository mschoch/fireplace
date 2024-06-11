package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type DataStoreFileSystem struct {
	rootDir string
}

func NewFileSystemDataStore(root string) *DataStoreFileSystem {
	return &DataStoreFileSystem{
		rootDir: root,
	}
}

func (s *DataStoreFileSystem) Get(key string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.rootDir, key)
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("error opening file %q:%w", fullPath, err)
	}

	return f, nil
}

func (s *DataStoreFileSystem) Set(key string, val io.ReadCloser) error {
	fullPath := filepath.Join(s.rootDir, key)
	dir, _ := filepath.Split(fullPath)
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return fmt.Errorf("error making parent directory %s: %w", dir, err)
	}

	var file *os.File
	file, err = os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("error uploading: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, val)
	if err != nil {
		return fmt.Errorf("error uploading: %w", err)
	}

	return val.Close()
}
