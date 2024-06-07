package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-kit/kit/log"
	kittransport "github.com/go-kit/kit/transport"
	kithttp "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"tailscale.com/client/tailscale"
)

// makeHandler returns a handler for the tracking service.
func makeHandlerForApplication(app *Application, ts Service, lc *tailscale.LocalClient, logger log.Logger) http.Handler {
	r := mux.NewRouter()

	opts := []kithttp.ServerOption{
		kithttp.ServerErrorHandler(kittransport.NewLogErrorHandler(logger)),
		kithttp.ServerErrorEncoder(encodeError),
	}

	whoHandler := kithttp.NewServer(
		makeWhoEndpoint(ts, lc),
		decodeWhoRequest,
		makeJSONResponseEncoder(logger),
		opts...,
	)

	databasesHandler := kithttp.NewServer(
		makeDatabasesEndpoint(ts),
		decodeEmptyRequest,
		makeJSONResponseEncoder(logger),
		opts...,
	)

	applicationsHandler := kithttp.NewServer(
		makeApplicationsEndpoint(ts, lc),
		decodeEmptyRequest,
		makeJSONResponseEncoder(logger),
		opts...,
	)

	setMetaHandler := kithttp.NewServer(
		makeSetMetaEndpoint(ts),
		decodeSetMetaRequest,
		makeJSONResponseEncoder(logger),
		opts...,
	)

	uploadGetHandler := kithttp.NewServer(
		makeUploadGetEndpoint(ts),
		decodeUploadGetRequest,
		makeJSONResponseEncoder(logger),
		opts...,
	)

	downloadHandler := kithttp.NewServer(
		makeDownloadEndpoint(ts),
		decodeDownloadRequest,
		makeStreamCARResponseEncoder(logger),
		opts...,
	)

	uploadDataHandler := kithttp.NewServer(
		makeUploadDataEndpoint(ts),
		decodeUploadDataRequest,
		makeJSONResponseEncoder(logger),
		opts...,
	)

	webSocketHandler := newWebsocketHandler(ts, logger)

	// fireproof AWS/s3 fake
	r.Handle("/api/download/data/{name}/{car}", downloadHandler).Methods("GET")
	r.Handle("/api/upload/data/{name}/{car}", uploadDataHandler).Methods("PUT")
	r.Handle("/api/upload", uploadGetHandler).Methods("GET")
	r.Handle("/api/upload", setMetaHandler).Methods("PUT")

	r.Handle("/api/websocket", webSocketHandler)

	// fireplace api
	r.Handle("/api/who", whoHandler).Methods("GET")
	r.Handle("/api/database", databasesHandler).Methods("GET")
	r.Handle("/api/application", applicationsHandler).Methods("GET")

	// static content, with SPA redirecting
	var mySPAHandler SPAFileSystem
	if filepath.Ext(app.LocalPath) == ".zip" {
		zr, err := zip.OpenReader(app.LocalPath)
		if err != nil {
			logger.Log("msg", "unable to open zip reader", "path", app.LocalPath, "err", err)
			return nil
		}
		logger.Log("msg", "app started with zip", "path", app.LocalPath)
		mySPAHandler = NewSPAFileSystem(zr, "index.html")
	} else {
		logger.Log("msg", "app started with dir", "path", app.LocalPath)
		mySPAHandler = NewSPAFileSystem(os.DirFS(app.LocalPath), "index.html")
	}
	r.PathPrefix("/").Handler(http.FileServerFS(mySPAHandler))

	return r
}

// encode errors from business-logic
func encodeError(_ context.Context, err error, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch err {
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}

func encodeJSONResponse(ctx context.Context, w http.ResponseWriter, response any) error {
	var err error
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch t := response.(type) {
	case error:
		w.WriteHeader(http.StatusInternalServerError)
		err = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": t.Error(),
		})
	default:
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(t)
	}

	return err
}

func streamCAR(ctx context.Context, w http.ResponseWriter, response io.ReadCloser) error {
	defer response.Close()
	w.Header().Set("Content-Type", "application/car")
	w.WriteHeader(http.StatusOK)
	_, err := io.Copy(w, response)
	return err
}
