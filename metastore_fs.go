package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type MetaStoreFileSystem struct {
	rootDir string

	mKnownDatabaseVersions sync.RWMutex
	knownDatabaseVersions  map[string]string
}

func NewFileSystemMetaStore(root string) *MetaStoreFileSystem {
	return &MetaStoreFileSystem{
		rootDir:               root,
		knownDatabaseVersions: make(map[string]string),
	}
}

func (m *MetaStoreFileSystem) Init() error {
	m.mKnownDatabaseVersions.Lock()
	defer m.mKnownDatabaseVersions.Unlock()
	return m.walkMetaDir()
}

func (m *MetaStoreFileSystem) walkMetaDir() error {
	err := filepath.Walk(m.rootDir, func(path string, info os.FileInfo, err error) error {
		mdk := MetaDataKey(info.Name())
		if info.IsDir() && mdk.Valid() {
			m.knownDatabaseVersions[mdk.Name()] = mdk.Version()
			return nil
		}
		return nil
	})
	return err
}

func (m *MetaStoreFileSystem) Set(name, branch string, meta *MetaRequest, metaRaw []byte) error {

	fullPath := filepath.Join(m.rootDir, name, meta.CID)

	dir, _ := filepath.Split(fullPath)
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return fmt.Errorf("error creating parent directory %q: %w", dir, err)
	}

	err = os.WriteFile(fullPath, metaRaw, 0666)
	if err != nil {
		return fmt.Errorf("error uploading: %w", err)
	}

	// FIXME now try to delete the parents

	// now update our in memory table
	mdk := MetaDataKey(name)
	if mdk.Valid() {
		m.mKnownDatabaseVersions.Lock()
		m.knownDatabaseVersions[mdk.Name()] = mdk.Version()
		m.mKnownDatabaseVersions.Unlock()
	}

	return nil
}

func (m *MetaStoreFileSystem) Meta(name, branch string) ([]*MetaRequest, error) {
	dirPath := filepath.Join(m.rootDir, name)
	items := make([]*MetaRequest, 0)
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil && os.IsNotExist(err) {
			// if dir doesn't exist, ignore
			return nil
		} else if err != nil {
			return err
		}
		if !info.IsDir() {
			fbytes, err2 := os.ReadFile(path)
			if err2 != nil {
				return err2
			}
			var mreq MetaRequest
			err2 = json.Unmarshal(fbytes, &mreq)
			if err2 != nil {
				return err2
			}
			items = append(items, &MetaRequest{CID: mreq.CID, Data: mreq.Data})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error walking meta dir: %w", err)
	}

	return items, nil
}

func (m *MetaStoreFileSystem) Databases() ([]*Database, error) {
	m.mKnownDatabaseVersions.RLock()
	defer m.mKnownDatabaseVersions.RUnlock()
	rv := make([]*Database, 0, len(m.knownDatabaseVersions))
	for name, version := range m.knownDatabaseVersions {
		rv = append(rv, &Database{
			Name:    name,
			Version: version,
		})
	}
	return rv, nil
}
