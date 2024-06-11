package main

import (
	"encoding/json"
	"testing"
)

func TestMetaStores(t *testing.T) {
	tests := []struct {
		name string
		m    MetaStore
	}{
		{
			name: "mem",
			m:    NewMemoryMetaStore(),
		},
		{
			name: "fs",
			m:    NewFileSystemMetaStore(t.TempDir()),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testMetaStore(t, test.m)
		})
	}
}

func testMetaStore(t *testing.T, m MetaStore) {

	// list dbs, expect no errors and no dbs
	dbs, err := m.Databases()
	if err != nil {
		t.Fatalf("error listing databases: %v", err)
	}
	if len(dbs) != 0 {
		t.Errorf("expected %d dbs, got %d", 0, len(dbs))
	}

	// try get meta for a name that should not exist
	mreqs, err := m.Meta("dne", "")
	if err != nil {
		t.Fatalf("error getting metadata: %v", err)
	}
	if len(mreqs) != 0 {
		t.Errorf("expected %d metarequests, got %d", 0, len(mreqs))
	}

	// set some metadata
	mreq := &MetaRequest{
		CID:  "cid",
		Data: "data",
	}
	err = m.Set("db1", "", mreq, jsonBytes(mreq))
	if err != nil {
		t.Fatalf("error setting metadata: %v", err)
	}

	// try getting that metadata back
	mreqs, err = m.Meta("db1", "")
	if err != nil {
		t.Fatalf("error getting metadata: %v", err)
	}
	if len(mreqs) != 1 {
		t.Fatalf("expected %d metarequests, got %d", 1, len(mreqs))
	}
	if mreqs[0].CID != mreq.CID {
		t.Errorf("expected cid %q, got cid %q", mreq.CID, mreqs[0].CID)
	}
	if mreqs[0].Data != mreq.Data {
		t.Errorf("expected data %q, got data %q", mreq.Data, mreqs[0].Data)
	}

	// set second metadata for the same db
	mreq2 := &MetaRequest{
		CID:  "cid2",
		Data: "data2",
	}
	err = m.Set("db1", "", mreq2, jsonBytes(mreq2))
	if err != nil {
		t.Fatalf("error setting metadata: %v", err)
	}
	// ensure we get 2 reqs back now
	mreqs, err = m.Meta("db1", "")
	if err != nil {
		t.Fatalf("error getting metadata: %v", err)
	}
	if len(mreqs) != 2 {
		t.Fatalf("expected %d metarequests, got %d", 2, len(mreqs))
	}

	// FIXME could add additional assertions
}

func jsonBytes(v any) []byte {
	rv, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return rv
}
