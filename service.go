package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dustin/go-broadcast"
	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"

	"github.com/go-kit/log"
)

type Service interface {
	PrepareData(name, car string) (*UploadRequestResponse, error)
	Data(name, car string) (io.ReadCloser, error)
	SetData(name, car string, body io.ReadCloser) error

	Meta(name, branch string) ([]*MetaRequest, error)
	SetMeta(name, branch string, meta *MetaRequest, metaRaw []byte) error
	RegisterMeta(chan interface{})
	UnregisterMeta(chan interface{})

	Databases() ([]*Database, error)
	Who(lc *tailscale.LocalClient, remoteAddr string) (*UserProfile, error)
	LaunchApplication(app *Application) error
	RunningApplications(lc *tailscale.LocalClient) ([]*Application, error)
}

type service struct {
	dataDir  string
	metaDir  string
	appsDir  string
	tsnetDir string

	logts bool

	log log.Logger

	mKnownDatabaseVersions sync.RWMutex
	knownDatabaseVersions  map[string]string

	mRunningApplications sync.RWMutex
	runningApplications  map[string]RunningApplication

	broadcaster broadcast.Broadcaster
}

func newService(l log.Logger, dataDir, metaDir, appsDir, tsnetDir string, logts bool) *service {
	return &service{
		log:      l,
		dataDir:  dataDir,
		metaDir:  metaDir,
		appsDir:  appsDir,
		tsnetDir: tsnetDir,
		logts:    logts,

		knownDatabaseVersions: make(map[string]string),
		runningApplications:   make(map[string]RunningApplication),

		broadcaster: broadcast.NewBroadcaster(100),
	}
}

func ensureDirectoriesExist(paths ...string) error {
	for _, path := range paths {
		err := os.MkdirAll(path, 0700)
		if err != nil {
			return fmt.Errorf("mkdirall %q error: %w", path, err)
		}
	}
	return nil
}

func (s *service) init() error {
	// ensure that the directories we require exist
	err := ensureDirectoriesExist(s.metaDir, s.dataDir, s.tsnetDir, s.appsDir)
	if err != nil {
		return fmt.Errorf("error ensuring required directory exists: %w", err)
	}

	s.mKnownDatabaseVersions.Lock()
	defer s.mKnownDatabaseVersions.Unlock()
	return s.walkMetaDir(s.metaDir)
}

func (s *service) walkMetaDir(root string) error {
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		mdk := MetaDataKey(info.Name())
		if info.IsDir() && mdk.Valid() {
			s.knownDatabaseVersions[mdk.Name()] = mdk.Version()
			return nil
		}
		return nil
	})
	return err
}

func (s *service) Who(lc *tailscale.LocalClient, remoteAddr string) (*UserProfile, error) {
	who, err := lc.WhoIs(context.Background(), remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("tailscale local client whois error: %w", err)
	}

	return &UserProfile{
		LoginName:     who.UserProfile.LoginName,
		DisplayName:   who.UserProfile.DisplayName,
		ProfilePicURL: who.UserProfile.ProfilePicURL,
		Node:          firstLabel(who.Node.ComputedName),
	}, nil
}

func firstLabel(s string) string {
	if hostname, _, ok := strings.Cut(s, "."); ok {
		return hostname
	}

	return s
}

func (s *service) Databases() ([]*Database, error) {
	s.mKnownDatabaseVersions.RLock()
	defer s.mKnownDatabaseVersions.RUnlock()
	rv := make([]*Database, 0, len(s.knownDatabaseVersions))
	for name, version := range s.knownDatabaseVersions {
		rv = append(rv, &Database{
			Name:    name,
			Version: version,
		})
	}
	return rv, nil
}

func (s *service) LaunchApplication(app *Application) error {
	appTSNetDir := filepath.Join(s.tsnetDir, app.Hostname)
	err := os.MkdirAll(appTSNetDir, 0700)
	if err != nil {
		return fmt.Errorf("mkdir all error: %v", err)
	}

	srv := &tsnet.Server{
		Hostname: app.Hostname,
		Dir:      appTSNetDir,
	}

	if !s.logts {
		srv.Logf = func(format string, args ...any) {

		}
	}

	lc, err := srv.LocalClient()
	if err != nil {
		return fmt.Errorf("error getting tsnet local client: %w", err)
	}

	ln, err := srv.ListenTLS("tcp", app.BindAddr)
	if err != nil {
		return fmt.Errorf("error listen tls: %w", err)
	}

	h := makeHandlerForApplication(app, s, lc, s.log)

	go func() {
		err2 := http.Serve(ln, h)
		if err2 != nil {
			s.log.Log("msg", "http serve for application %q returned %w", app.Name, err2)
		}
	}()

	s.mRunningApplications.Lock()
	defer s.mRunningApplications.Unlock()
	s.runningApplications[app.Name] = RunningApplication{
		server:   srv,
		listener: ln,
		app:      app,
	}

	return nil
}

type RunningApplication struct {
	server   *tsnet.Server
	listener net.Listener

	app *Application
}

func (s *service) RunningApplications(lc *tailscale.LocalClient) ([]*Application, error) {
	s.mRunningApplications.RLock()
	defer s.mRunningApplications.RUnlock()
	rv := make([]*Application, 0, len(s.runningApplications))
	for _, v := range s.runningApplications {
		status, err := lc.Status(context.Background())
		if err == nil && status != nil && status.CurrentTailnet != nil {
			v.app.URL = fmt.Sprintf("https://%s.%s%s/", v.app.Hostname, status.CurrentTailnet.MagicDNSSuffix, v.app.BindAddr)
		}
		rv = append(rv, v.app)
	}
	return rv, nil
}

func (s *service) SetMeta(name, branch string, meta *MetaRequest, metaRaw []byte) error {
	fullPath := filepath.Join(s.metaDir, name, meta.CID)

	dir, _ := filepath.Split(fullPath)
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return fmt.Errorf("error uploading: %w", err)
	}

	err = os.WriteFile(fullPath, metaRaw, 0666)
	if err != nil {
		return fmt.Errorf("error uploading: %w", err)
	}

	// FIXME now try to delete the parents

	// now update our in memory table
	mdk := MetaDataKey(name)
	if mdk.Valid() {
		s.mKnownDatabaseVersions.Lock()
		s.knownDatabaseVersions[mdk.Name()] = mdk.Version()
		s.mKnownDatabaseVersions.Unlock()
	}

	// finally broadcast this to other connected parties
	metaItems := &MetaItems{
		Items: []*MetaRequest{
			meta,
		},
	}
	s.broadcaster.Submit(metaItems)

	return nil
}

func (s *service) RegisterMeta(webSocket chan interface{}) {
	s.broadcaster.Register(webSocket)
}

func (s *service) UnregisterMeta(webSocket chan interface{}) {
	s.broadcaster.Unregister(webSocket)
}

func (s *service) Meta(name, branch string) ([]*MetaRequest, error) {
	dirPath := filepath.Join(s.metaDir, name)
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

func (s *service) PrepareData(name, car string) (*UploadRequestResponse, error) {
	uploadURL := fmt.Sprintf("/api/upload/data/%s/%s.car", name, car)
	uploadKey := fmt.Sprintf("data/%s/%s.car", name, car)
	rv := UploadRequestResponse{
		UploadURL: uploadURL,
		Key:       uploadKey,
	}
	return &rv, nil
}

func (s *service) Data(name, car string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.dataDir, name, car)
	carFile, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("error opening car file %q:%w", fullPath, err)
	}

	return carFile, nil
}

func (s *service) SetData(name, car string, body io.ReadCloser) error {
	fullPath := filepath.Join(s.dataDir, name, car)

	dir, _ := filepath.Split(fullPath)
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return fmt.Errorf("error uploading: %w", err)
	}

	var file *os.File
	file, err = os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("error uploading: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, body)
	if err != nil {
		return fmt.Errorf("error uploading: %w", err)
	}

	return nil
}
