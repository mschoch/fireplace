package main

import (
	"io/fs"
	"os"
)

// SPAFileSystem wraps another FileSystem, but returns a specified page instead of 404
type SPAFileSystem struct {
	index string
	fs.FS
}

func NewSPAFileSystem(parent fs.FS, index string) SPAFileSystem {
	return SPAFileSystem{
		FS:    parent,
		index: index,
	}
}

// Open is a wrapper around the Open method of the embedded FileSystem
// that serves a 403 permission error when name has a file or directory
// with whose name starts with a period in its path.
func (s SPAFileSystem) Open(name string) (fs.File, error) {
	file, err := s.FS.Open(name)

	// if the path was not found, return spa index
	if os.IsNotExist(err) {
		return s.FS.Open(s.index)
	} else if err != nil {
		return nil, err
	}

	return file, nil
}
