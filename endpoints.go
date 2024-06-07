package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
	"github.com/gorilla/mux"
	"tailscale.com/client/tailscale"
)

type emptyRequest struct{}

func decodeEmptyRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return emptyRequest{}, nil
}

type emptyResponse struct {
	Err error `json:"error,omitempty"`
}

type whoRequest struct {
	remoteAddr string
}

func decodeWhoRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	return whoRequest{
		remoteAddr: r.RemoteAddr,
	}, nil
}

type jsonResponse struct {
	emptyResponse
	Res any
}

type streamResponse struct {
	emptyResponse
	Res io.ReadCloser
}

func makeWhoEndpoint(s Service, lc *tailscale.LocalClient) endpoint.Endpoint {
	return func(_ context.Context, request interface{}) (interface{}, error) {
		req := request.(whoRequest)
		userProfile, err := s.Who(lc, req.remoteAddr)
		return jsonResponse{
			emptyResponse: emptyResponse{Err: err},
			Res:           userProfile,
		}, nil
	}
}

func makeDatabasesEndpoint(s Service) endpoint.Endpoint {
	return func(_ context.Context, request interface{}) (interface{}, error) {
		databases, err := s.Databases()
		return jsonResponse{
			emptyResponse: emptyResponse{Err: err},
			Res:           databases,
		}, nil
	}
}

func makeApplicationsEndpoint(s Service, lc *tailscale.LocalClient) endpoint.Endpoint {
	return func(_ context.Context, request interface{}) (interface{}, error) {
		apps, err := s.RunningApplications(lc)
		return jsonResponse{
			emptyResponse: emptyResponse{Err: err},
			Res:           apps,
		}, nil
	}
}

func makeJSONResponseEncoder(logger log.Logger) func(ctx context.Context, w http.ResponseWriter, res any) error {
	return func(ctx context.Context, w http.ResponseWriter, response any) error {
		res := response.(jsonResponse)
		if res.Err != nil {
			return encodeJSONResponse(ctx, w, res.Err)
		}
		// use res == nil to signal we want 201 CREATED instead
		if res.Res == nil {
			w.WriteHeader(http.StatusCreated)
			return nil
		}
		return encodeJSONResponse(ctx, w, res.Res)
	}
}

func makeStreamCARResponseEncoder(logger log.Logger) func(ctx context.Context, w http.ResponseWriter, res any) error {
	return func(ctx context.Context, w http.ResponseWriter, response any) error {
		res := response.(streamResponse)
		if res.Err != nil {
			return encodeJSONResponse(ctx, w, res.Err)
		}
		return streamCAR(ctx, w, res.Res)
	}
}

type uploadDataRequest struct {
	name string
	car  string
	data io.ReadCloser
}

func decodeUploadDataRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	name, ok := vars["name"]
	if !ok {
		return nil, fmt.Errorf("download request missing name")
	}
	car, ok := vars["car"]
	if !ok {
		return nil, fmt.Errorf("download request missing car")
	}

	return uploadDataRequest{
		name: name,
		car:  car,
		data: r.Body,
	}, nil
}

func makeUploadDataEndpoint(s Service) endpoint.Endpoint {
	return func(_ context.Context, request interface{}) (interface{}, error) {
		req := request.(uploadDataRequest)
		err := s.SetData(req.name, req.car, req.data)

		return jsonResponse{
			emptyResponse: emptyResponse{Err: err},
		}, nil
	}
}

type downloadRequest struct {
	name string
	car  string
}

func decodeDownloadRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	name, ok := vars["name"]
	if !ok {
		return nil, fmt.Errorf("download request missing name")
	}
	car, ok := vars["car"]
	if !ok {
		return nil, fmt.Errorf("download request missing car")
	}

	return downloadRequest{
		name: name,
		car:  car,
	}, nil
}

func makeDownloadEndpoint(s Service) endpoint.Endpoint {
	return func(_ context.Context, request interface{}) (interface{}, error) {
		req := request.(downloadRequest)
		data, err := s.Data(req.name, req.car)

		return streamResponse{
			emptyResponse: emptyResponse{Err: err},
			Res:           data,
		}, nil
	}
}

type uploadGetRequest struct {
	typ    string
	name   string
	branch string
	car    string
}

func decodeUploadGetRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %w", err)
	}
	var metaRequest MetaRequest
	err = json.Unmarshal(bodyBytes, &metaRequest)

	return uploadGetRequest{
		typ:    r.FormValue("type"),
		name:   r.FormValue("name"),
		branch: r.FormValue("branch"),
		car:    r.FormValue("car"),
	}, nil
}

func makeUploadGetEndpoint(s Service) endpoint.Endpoint {
	return func(_ context.Context, request interface{}) (interface{}, error) {
		req := request.(uploadGetRequest)

		var err error
		var res any
		switch req.typ {
		case "meta":
			var metaItems []*MetaRequest
			metaItems, err = s.Meta(req.name, req.branch)
			wrapper := MetaItems{
				Items: metaItems,
			}

			itemsJSON, err := json.Marshal(wrapper)
			if err != nil {
				return nil, fmt.Errorf("error building items wrapper json: %w", err)
			}

			res = MetaResponse{
				Status: 200,
				Body:   string(itemsJSON),
			}
		case "data":
			res, err = s.PrepareData(req.name, req.car)
		default:
			return nil, fmt.Errorf("unsupported upload type: %s", req.typ)
		}

		return jsonResponse{
			emptyResponse: emptyResponse{Err: err},
			Res:           res,
		}, nil
	}
}

type setMetaRequest struct {
	typ    string
	name   string
	branch string
	meta   *MetaRequest
	raw    []byte
}

func decodeSetMetaRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body: %w", err)
	}
	var metaRequest MetaRequest
	err = json.Unmarshal(bodyBytes, &metaRequest)

	return setMetaRequest{
		typ:    r.FormValue("type"),
		name:   r.FormValue("name"),
		branch: r.FormValue("branch"),
		meta:   &metaRequest,
		raw:    bodyBytes,
	}, nil
}

func makeSetMetaEndpoint(s Service) endpoint.Endpoint {
	return func(_ context.Context, request interface{}) (interface{}, error) {
		req := request.(setMetaRequest)
		err := s.SetMeta(req.name, req.branch, req.meta, req.raw)
		return jsonResponse{
			emptyResponse: emptyResponse{Err: err},
			Res: MetaResponse{
				Status: 201,
				Body:   `{}`,
			},
		}, nil
	}
}
