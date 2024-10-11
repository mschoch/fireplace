package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/go-kit/log"
)

func TestDownloadDataHandler(t *testing.T) {
	_, svc, _, handler := setupTest(t)

	dbName := "db1"
	dbKey := DataKeyFromDatabase(dbName)
	sum := sha256.Sum256([]byte("fireplace"))
	car := fmt.Sprintf("%x.car", sum)
	contents := "TOPSECRET"

	err := svc.SetData(string(dbKey), car, readCloserString(contents))
	if err != nil {
		t.Fatalf("error creating test data: %v", err)
	}

	target := fmt.Sprintf("/api/download/data/%s/%s", dbKey, car)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d got %d", http.StatusOK, rr.Code)
	}

	data, err := io.ReadAll(rr.Body)
	if err != nil {
		t.Fatalf("error reading response body: %v", err)
	}
	if string(data) != contents {
		t.Errorf("expected %q, got %q", contents, string(data))
	}
}

func TestUploadDataHandler(t *testing.T) {
	_, svc, _, handler := setupTest(t)

	dbName := "db1"
	dbKey := DataKeyFromDatabase(dbName)
	sum := sha256.Sum256([]byte("fireplace"))
	car := fmt.Sprintf("%x.car", sum)
	contents := "TOPSECRET"

	target := fmt.Sprintf("/api/upload/data/%s/%s", dbKey, car)
	req := httptest.NewRequest(http.MethodPut, target, readCloserString(contents))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("expected status %d got %d", http.StatusCreated, rr.Code)
	}

	actual, err := stringErrReadCloserErr(svc.Data(string(dbKey), car))
	if err != nil {
		t.Fatalf("error reading test data: %v", err)
	}
	if actual != contents {
		t.Errorf("expected %q, got %q", contents, actual)
	}

}

func TestPrepareUploadData(t *testing.T) {

	_, _, _, handler := setupTest(t)

	dbName := "db1"
	dbKey := DataKeyFromDatabase(dbName)
	sum := sha256.Sum256([]byte("fireplace"))
	carNoExt := fmt.Sprintf("%x", sum)

	prepareUploadTarget := dataPrepareUploadRequest(string(dbKey), carNoExt)
	req := httptest.NewRequest(http.MethodGet, prepareUploadTarget, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d got %d", http.StatusOK, rr.Code)
	}

	var res UploadRequestResponse
	err := json.NewDecoder(rr.Body).Decode(&res)
	if err != nil {
		t.Fatalf("error decoding json: %v", err)
	}
	expectURL := fmt.Sprintf("/api/upload/data/%s/%s.car", dbKey, carNoExt)
	if res.UploadURL != expectURL {
		t.Errorf("expected %q, got %q", expectURL, res.UploadURL)
	}
	expectKey := fmt.Sprintf("data/%s/%s.car", dbKey, carNoExt)
	if res.Key != expectKey {
		t.Errorf("expected %q, got %q", expectKey, res.Key)
	}
}

func TestDownloadMeta(t *testing.T) {
	_, svc, _, handler := setupTest(t)

	dbName := "db1"
	dbKey := MetaDataKeyFromDatabaseVersion(dbName, "0.18")
	sum := sha256.Sum256([]byte("fireplace"))
	carNoExt := fmt.Sprintf("%x", sum)
	contents := `{"car": {"/":"bafkreiajkcu646s3w522spmlfuuhm67dke6jayqvgj7r5mir5ruhmpaa5y"}, "key": "key"}`

	mr := &MetaRequest{
		CID:     carNoExt,
		Data:    base64.StdEncoding.EncodeToString([]byte(contents)),
		Parents: nil,
	}
	mrBytes, err := json.Marshal(mr)
	if err != nil {
		t.Fatalf("error encoding json: %v", err)
	}
	err = svc.SetMeta(string(dbKey), "", mr, mrBytes)
	if err != nil {
		t.Fatalf("error setting meta for test: %v", err)
	}

	getMetaTarget := metaGetRequest(string(dbKey), "")
	req := httptest.NewRequest(http.MethodGet, getMetaTarget, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d got %d", http.StatusOK, rr.Code)
	}

	var res MetaResponse
	err = json.NewDecoder(rr.Body).Decode(&res)
	if err != nil {
		t.Fatalf("error decoding json: %v", err)
	}
	if res.Status != 200 {
		t.Errorf("expected meta response status 200, got %d", res.Status)
	}

	var inner MetaItems
	err = json.Unmarshal([]byte(res.Body), &inner)
	if err != nil {
		t.Fatalf("error decoding inner json: %v", err)
	}

	if len(inner.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(inner.Items))
	}

	if inner.Items[0].CID != mr.CID {
		t.Errorf("expected CID %q got %q", mr.CID, inner.Items[0].CID)
	}
	if inner.Items[0].Data != mr.Data {
		t.Errorf("expeced data %q got %q", mr.Data, inner.Items[0].Data)
	}
}

func TestUploadMeta(t *testing.T) {
	_, svc, _, handler := setupTest(t)

	dbName := "db1"
	dbKey := MetaDataKeyFromDatabaseVersion(dbName, "0.18")
	sum := sha256.Sum256([]byte("fireplace"))
	carNoExt := fmt.Sprintf("%x", sum)
	contents := `{"car": {"/":"bafkreiajkcu646s3w522spmlfuuhm67dke6jayqvgj7r5mir5ruhmpaa5y"}, "key": "key"}`

	mr := &MetaRequest{
		CID:     carNoExt,
		Data:    base64.StdEncoding.EncodeToString([]byte(contents)),
		Parents: nil,
	}
	mrBytes, err := json.Marshal(mr)
	if err != nil {
		t.Fatalf("error encoding json: %v", err)
	}

	getMetaTarget := metaGetRequest(string(dbKey), "")
	req := httptest.NewRequest(http.MethodPut, getMetaTarget, bytes.NewBuffer(mrBytes))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d got %d", http.StatusOK, rr.Code)
	}

	metaReqs, err := svc.Meta(string(dbKey), "")
	if err != nil {
		t.Fatalf("error loading meta data: %v", err)
	}
	if len(metaReqs) != 1 {
		t.Fatalf("expected 1 meta, got %d", len(metaReqs))
	}

	if metaReqs[0].CID != mr.CID {
		t.Errorf("expected CID %q got %q", mr.CID, metaReqs[0].CID)
	}
	if metaReqs[0].Data != mr.Data {
		t.Errorf("expeced data %q got %q", mr.Data, metaReqs[0].Data)
	}
}

func metaGetRequest(name, branch string) string {
	u := url.URL{
		Path: "/api/upload",
	}
	q := u.Query()
	q.Set("type", "meta")
	q.Set("name", name)
	q.Set("branch", branch)
	u.RawQuery = q.Encode()

	return u.String()
}

func dataPrepareUploadRequest(name, car string) string {
	u := url.URL{
		Path: "/api/upload",
	}
	q := u.Query()
	q.Set("type", "data")
	q.Set("name", name)
	q.Set("car", car)
	u.RawQuery = q.Encode()

	return u.String()
}

func setupTest(t *testing.T) (log.Logger, *service, *Application, http.Handler) {
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	dataStore := NewMemoryDataStore()
	metaStore := NewMemoryMetaStore(logger)
	svc := newService(logger, dataStore, metaStore, "", "", false)

	testApp := &Application{
		Name:      "testapp",
		AutoStart: false,
		LocalPath: t.TempDir(),
	}

	handler := makeHandlerForApplication(testApp, svc, nil, logger)

	return logger, svc, testApp, handler
}
