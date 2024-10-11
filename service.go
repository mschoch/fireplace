package main

import (
	"context"
	"encoding/base64"
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
	appsDir  string
	tsnetDir string

	dataStore DataStore
	metaStore MetaStore

	logts bool

	log log.Logger

	mRunningApplications sync.RWMutex
	runningApplications  map[string]RunningApplication

	broadcaster broadcast.Broadcaster
}

func newService(l log.Logger, dataStore DataStore, metaStore MetaStore, appsDir, tsnetDir string, logts bool) *service {
	return &service{
		log:      l,
		appsDir:  appsDir,
		tsnetDir: tsnetDir,
		logts:    logts,

		dataStore: dataStore,
		metaStore: metaStore,

		runningApplications: make(map[string]RunningApplication),

		broadcaster: broadcast.NewBroadcaster(100),
	}
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
	return s.metaStore.Databases()
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
	err := s.metaStore.Set(name, branch, meta, metaRaw)
	if err != nil {
		return fmt.Errorf("error storing metadata: %w", err)
	}

	dataBytes, err := base64.StdEncoding.DecodeString(meta.Data)
	if err != nil {
		return fmt.Errorf("error decoding meta data base64: %v", err)
	}
	s.log.Log("msg", "decoded base64", "databytes", string(dataBytes))

	var dbm DBMeta
	err = json.Unmarshal(dataBytes, &dbm)
	if err != nil {
		return fmt.Errorf("error decoding db meta: %v", err)
	}

	s.log.Log("msg", "parsed db meta", "car codec", dbm.CAR, "key", dbm.Key)

	//parsedCID, err := cid.Decode(dbm.CAR)
	//if err != nil {
	//	return fmt.Errorf("msg", "error decoding car", "err", err)
	//}
	//s.log.Log("msg", "parsed cid", "it", fmt.Sprintf("%#v", parsedCID))

	//dataBuf := bytes.NewBuffer(dataBytes)
	//nb := basicnode.Prototype.Map.NewBuilder()
	//err = dagjson.Decode(nb, dataBuf)
	//if err != nil {
	//	s.log.Log("msg", "error decoding meta dag json", "err", err)
	//	//return fmt.Errorf("error decoding meta dag json: %v", err)
	//} else {
	//	node := nb.Build()
	//	node.
	//		s.log.Log("msg", "no err decoding meta dag json", "built", fmt.Sprintf("%#v", node))
	//}

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
	return s.metaStore.Meta(name, branch)
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
	return s.dataStore.Get(filepath.Join(name, car))
}

func (s *service) SetData(name, car string, body io.ReadCloser) error {
	return s.dataStore.Set(filepath.Join(name, car), body)
}
