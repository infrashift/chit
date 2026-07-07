package api

import (
	"net/http"
	"testing"
)

type recordTransport struct {
	lastReq *http.Request
}

func (rt *recordTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.lastReq = req
	return &http.Response{StatusCode: 200}, nil
}

func TestAuthTransport_InjectsToken(t *testing.T) {
	recorder := &recordTransport{}
	transport := newAuthTransport(recorder, func() string { return "my-token" })

	req, _ := http.NewRequest("GET", "http://example.com/api/v1/users/me", nil)
	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}

	got := recorder.lastReq.Header.Get("X-Session-Token")
	if got != "my-token" {
		t.Errorf("X-Session-Token = %q, want %q", got, "my-token")
	}
}

func TestAuthTransport_ClonesRequest(t *testing.T) {
	recorder := &recordTransport{}
	transport := newAuthTransport(recorder, func() string { return "tok" })

	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}

	// Original request should NOT have the header.
	if req.Header.Get("X-Session-Token") != "" {
		t.Error("original request was mutated")
	}
	// Cloned request should have the header.
	if recorder.lastReq.Header.Get("X-Session-Token") != "tok" {
		t.Error("cloned request missing token")
	}
}

func TestNewAuthTransport_DefaultBase(t *testing.T) {
	transport := newAuthTransport(nil, func() string { return "tok" })
	if transport.base != http.DefaultTransport {
		t.Error("expected http.DefaultTransport as base")
	}
}

func TestAuthTransportWithHeader_UsesCustomHeader(t *testing.T) {
	recorder := &recordTransport{}
	transport := newAuthTransportWithHeader(recorder, func() string { return "kratos-id" }, "X-User-Id")

	req, _ := http.NewRequest("GET", "http://example.com/api/v1/users/me", nil)
	_, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}

	got := recorder.lastReq.Header.Get("X-User-Id")
	if got != "kratos-id" {
		t.Errorf("X-User-Id = %q, want %q", got, "kratos-id")
	}
	if recorder.lastReq.Header.Get("X-Session-Token") != "" {
		t.Error("X-Session-Token should not be set when using custom header")
	}
}
