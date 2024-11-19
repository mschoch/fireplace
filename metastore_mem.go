package main

import (
	"sync"

	"github.com/go-kit/log"
)

var _ MetaStore = (*MetaStoreMemory)(nil)

type metaReqsByCID map[string]*MetaRequest

type metaReqsByCIDByName map[string]metaReqsByCID

type MetaStoreMemory struct {
	m              sync.RWMutex
	meta           metaReqsByCIDByName
	knownDatabases map[string]struct{}
	log            log.Logger
}

func NewMemoryMetaStore(log log.Logger) *MetaStoreMemory {
	return &MetaStoreMemory{
		meta:           make(metaReqsByCIDByName),
		knownDatabases: make(map[string]struct{}),
		log:            log,
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

	for _, parent := range meta.Parents {
		err := m.Delete(name, branch, parent)
		if err != nil {
			m.log.Log("msg", "error deleting parent, name:%q cid:%q", name, parent)
		}
	}

	// now update our in memory table
	m.knownDatabases[name] = struct{}{}

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
	rv := make([]*Database, 0, len(m.knownDatabases))
	for name := range m.knownDatabases {
		rv = append(rv, &Database{
			Name: name,
		})
	}
	return rv, nil
}

func (m *MetaStoreMemory) Delete(name, branch, cid string) error {
	m.m.Lock()
	defer m.m.Unlock()

	byName, nameExists := m.meta[name]
	if nameExists {
		delete(byName, cid)
	}

	return nil
}
