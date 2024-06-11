package main

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

type DataStoreMemory struct {
	m    sync.RWMutex
	data map[string][]byte
}

func NewMemoryDataStore() *DataStoreMemory {
	return &DataStoreMemory{
		data: make(map[string][]byte),
	}
}

func (s *DataStoreMemory) Get(key string) (io.ReadCloser, error) {
	s.m.RLock()
	defer s.m.RUnlock()
	if val, ok := s.data[key]; ok {
		return io.NopCloser(bytes.NewBuffer(val)), nil
	}

	return nil, nil
}

func (s *DataStoreMemory) Set(key string, val io.ReadCloser) error {
	s.m.Lock()
	defer s.m.Unlock()
	realizedValue, err := io.ReadAll(val)
	if err != nil {
		return fmt.Errorf("error reading value: %w", err)
	}

	s.data[key] = realizedValue

	return val.Close()
}
