package main

import (
	"fmt"
	"io"
	"time"

	"github.com/go-kit/kit/log"
	"tailscale.com/client/tailscale"
)

type Middleware func(Service) Service

func newLoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return &loggingMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

type loggingMiddleware struct {
	next   Service
	logger log.Logger
}

func (l *loggingMiddleware) Who(lc *tailscale.LocalClient, remoteAddr string) (userProfile *UserProfile, err error) {
	defer func(begin time.Time) {
		l.logger.Log("method", "Who", "remoteAddr", remoteAddr, "took", time.Since(begin), "err", err)
	}(time.Now())
	return l.next.Who(lc, remoteAddr)
}

func (l *loggingMiddleware) Databases() (d []*Database, err error) {
	defer func(begin time.Time) {
		l.logger.Log("method", "Databases", "took", time.Since(begin), "err", err)
	}(time.Now())
	return l.next.Databases()
}

func (l *loggingMiddleware) LaunchApplication(app *Application) (err error) {
	defer func(begin time.Time) {
		l.logger.Log("method", "LaunchApplication", "app", fmt.Sprintf("%#v", app), "took", time.Since(begin), "err", err)
	}(time.Now())
	return l.next.LaunchApplication(app)
}

func (l *loggingMiddleware) RunningApplications(lc *tailscale.LocalClient) (apps []*Application, err error) {
	defer func(begin time.Time) {
		l.logger.Log("method", "RunningApplications", "took", time.Since(begin), "err", err)
	}(time.Now())
	return l.next.RunningApplications(lc)
}

func (l *loggingMiddleware) SetMeta(name, branch string, meta *MetaRequest, metaRaw []byte) (err error) {
	defer func(begin time.Time) {
		l.logger.Log("method", "SetMeta", "name", name, "branch", branch, "took", time.Since(begin), "err", err)
	}(time.Now())
	return l.next.SetMeta(name, branch, meta, metaRaw)
}

func (l *loggingMiddleware) RegisterMeta(webSocket chan interface{}) {
	defer func(begin time.Time) {
		l.logger.Log("method", "RegisterMeta", "took", time.Since(begin))
	}(time.Now())
	l.next.RegisterMeta(webSocket)
}

func (l *loggingMiddleware) UnregisterMeta(webSocket chan interface{}) {
	defer func(begin time.Time) {
		l.logger.Log("method", "UnregisterMeta", "took", time.Since(begin))
	}(time.Now())
	l.next.UnregisterMeta(webSocket)
}

func (l *loggingMiddleware) Meta(name, branch string) (m []*MetaRequest, err error) {
	defer func(begin time.Time) {
		l.logger.Log("method", "Meta", "name", name, "branch", branch, "took", time.Since(begin), "err", err)
	}(time.Now())
	return l.next.Meta(name, branch)
}

func (l *loggingMiddleware) PrepareData(name, car string) (u *UploadRequestResponse, err error) {
	defer func(begin time.Time) {
		l.logger.Log("method", "PrepareData", "name", name, "car", car, "took", time.Since(begin), "err", err)
	}(time.Now())
	return l.next.PrepareData(name, car)
}

func (l *loggingMiddleware) Data(name, car string) (r io.ReadCloser, err error) {
	defer func(begin time.Time) {
		l.logger.Log("method", "Data", "name", name, "car", car, "took", time.Since(begin), "err", err)
	}(time.Now())
	return l.next.Data(name, car)
}

func (l *loggingMiddleware) SetData(name, car string, body io.ReadCloser) (err error) {
	defer func(begin time.Time) {
		l.logger.Log("method", "SetData", "name", name, "car", car, "took", time.Since(begin), "err", err)
	}(time.Now())
	return l.next.SetData(name, car, body)
}
