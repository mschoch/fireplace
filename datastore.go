package main

import "io"

type DataStore interface {
	Set(key string, r io.ReadCloser) error
	Get(key string) (io.ReadCloser, error)
}
