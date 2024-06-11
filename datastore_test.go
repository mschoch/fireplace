package main

import (
	"bytes"
	"io"
	"testing"
)

func TestDataStores(t *testing.T) {
	tests := []struct {
		name string
		d    DataStore
	}{
		{
			name: "mem",
			d:    NewMemoryDataStore(),
		},
		{
			name: "fs",
			d:    NewFileSystemDataStore(t.TempDir()),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testDataStore(t, test.d)
		})
	}
}

func testDataStore(t *testing.T, d DataStore) {

	// set a key
	err := d.Set("a", readCloserString("val-a"))
	if err != nil {
		t.Errorf("error setting key: %v", err)
	}

	// make sure we can read it back
	var aval string
	aval, err = stringErrReadCloserErr(d.Get("a"))
	if err != nil {
		t.Errorf("error getting key: %v", err)
	}
	if aval != "val-a" {
		t.Errorf("expected %q, got %q", "val-a", aval)
	}

	// set a different key
	err = d.Set("b", readCloserString("val-b"))
	if err != nil {
		t.Errorf("error setting key: %v", err)
	}

	// make sure we can read it back too
	var bval string
	bval, err = stringErrReadCloserErr(d.Get("b"))
	if err != nil {
		t.Errorf("error getting key: %v", err)
	}
	if bval != "val-b" {
		t.Errorf("expected %q, got %q", "val-b", aval)
	}

	// make sure we can still read the first key
	aval, err = stringErrReadCloserErr(d.Get("a"))
	if err != nil {
		t.Errorf("error getting key: %v", err)
	}
	if aval != "val-a" {
		t.Errorf("expected %q, got %q", "val-a", aval)
	}
}

func readCloserString(s string) io.ReadCloser {
	return io.NopCloser(bytes.NewBuffer([]byte(s)))
}

func stringErrReadCloserErr(r io.ReadCloser, err error) (string, error) {
	if err != nil {
		return "", err
	}
	rv, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}

	return string(rv), r.Close()
}
