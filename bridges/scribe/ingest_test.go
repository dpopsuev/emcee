package scribe_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bridge "github.com/dpopsuev/emcee/bridges/scribe"
	"github.com/dpopsuev/emcee/testdata"
)

func TestIngestIssues_PostsNDJSON(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s; want POST", r.Method)
		}
		if !strings.Contains(r.URL.String(), "source=emcee") {
			t.Errorf("URL = %s; want source=emcee param", r.URL.String())
		}
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.WriteHeader(http.StatusMultiStatus)
	}))
	defer srv.Close()

	err := bridge.IngestIssues(context.Background(), testdata.SampleIssues(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "jira:AUTH-42") {
		t.Error("body missing jira:AUTH-42")
	}
	if !strings.Contains(body, `"type":"node"`) {
		t.Error("body missing node records")
	}
	if !strings.Contains(body, `"type":"meta"`) {
		t.Error("body missing meta record")
	}
}

func TestIngestTriageGraph_PostsEdges(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.WriteHeader(http.StatusMultiStatus)
	}))
	defer srv.Close()

	err := bridge.IngestTriageGraph(context.Background(), testdata.SampleTriageGraph(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `"type":"edge"`) {
		t.Error("body missing edge records")
	}
	if !strings.Contains(body, "fixed_by") {
		t.Error("body missing fixed_by relation")
	}
}

func TestIngestIssues_ServerErrorReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := bridge.IngestIssues(context.Background(), testdata.SampleIssues(), srv.URL)
	if err == nil {
		t.Error("expected error on 500")
	}
}
