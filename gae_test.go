package nicehttp

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestParseRules(t *testing.T) {
	rules, err := parseRules("gae_test.yaml")
	if err != nil {
		t.Errorf("could not parse yaml: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("expected single rule, was: %v", len(rules))
	}

	expected := handlerRule{
		URL:       "/foo",
		StaticDir: "testdata",
		HTTPHeaders: map[string]string{
			"X-Test": "123",
		},
	}
	if !reflect.DeepEqual(expected, rules[0]) {
		t.Errorf("expected %+v, was %+v", expected, rules[0])
	}
}

func TestBuildHandler(t *testing.T) {
	rules := []handlerRule{
		handlerRule{
			URL:       "/foo",
			StaticDir: "gae/testdata",
		},
		handlerRule{
			URL:         "/bar/(.*)\\.md",
			StaticFiles: "gae/testdata/\\1.txt",
			MimeType:    "application/json",
		},
	}

	handler, err := buildHandler(rules, http.DefaultServeMux)
	if err != nil {
		t.Errorf("got err building handlers: %v", err)
	}

	var req *http.Request
	var rec *httptest.ResponseRecorder
	var resp *http.Response

	req = httptest.NewRequest("GET", "/foo/test.txt", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp = rec.Result()
	if resp.StatusCode != 200 {
		t.Errorf("expected valid status code for /foo/test.txt")
	}

	req = httptest.NewRequest("GET", "/bar/test.md", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp = rec.Result()
	if resp.StatusCode != 200 {
		t.Errorf("expected valid status code for /bar/test.md")
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("content-type not set correctly for /bar/test.md")
	}
}
