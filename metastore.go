package main

type MetaStore interface {
	Set(name, branch string, meta *MetaRequest, metaRaw []byte) error
	Meta(name, branch string) ([]*MetaRequest, error)

	Databases() ([]*Database, error)
}
