package main

import (
	"sync"
)

type metaReqsByCID map[string]*MetaRequest

type metaReqsByCIDByName map[string]metaReqsByCID

type MetaStoreMemory struct {
	m                     sync.RWMutex
	meta                  metaReqsByCIDByName
	knownDatabaseVersions map[string]string
}

func NewMemoryMetaStore() *MetaStoreMemory {
	return &MetaStoreMemory{
		meta:                  make(metaReqsByCIDByName),
		knownDatabaseVersions: make(map[string]string),
	}
}

func (m *MetaStoreMemory) Init() error {
	return nil
}

func (m *MetaStoreMemory) Set(name, branch string, meta *MetaRequest, _ []byte) error {
	m.m.Lock()
	defer m.m.Unlock()

	byName, nameExists := m.meta[name]
	if nameExists {
		byName[meta.CID] = meta
	} else {
		metaReqByCID := metaReqsByCID{
			meta.CID: meta,
		}
		m.meta[name] = metaReqByCID
	}

	// FIXME now try to delete the parents

	// now update our in memory table
	mdk := MetaDataKey(name)
	if mdk.Valid() {
		m.knownDatabaseVersions[mdk.Name()] = mdk.Version()
	}

	return nil
}

func (m *MetaStoreMemory) Meta(name, branch string) ([]*MetaRequest, error) {
	m.m.RLock()
	defer m.m.RUnlock()

	var items []*MetaRequest
	byName, nameExists := m.meta[name]
	if nameExists {
		for _, mreq := range byName {
			items = append(items, mreq)
		}
	}

	return items, nil
}

func (m *MetaStoreMemory) Databases() ([]*Database, error) {
	m.m.RLock()
	defer m.m.RUnlock()
	rv := make([]*Database, 0, len(m.knownDatabaseVersions))
	for name, version := range m.knownDatabaseVersions {
		rv = append(rv, &Database{
			Name:    name,
			Version: version,
		})
	}
	return rv, nil
}
