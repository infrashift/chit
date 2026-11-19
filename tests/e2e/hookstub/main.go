// Command hookstub is a membership policy hook for the stack e2e
// (tests/e2e/test_e2e.sh scenario 12). It stands in for HALO's halod.
//
// At start it registers chitd's OAuth2 client in Hydra, so that chitd can get
// a client_credentials token. It then answers POST /membership-check, after
// it checks the bearer token by Hydra introspection: the token must be active
// and belong to that client. It allows every member except the one user that
// the e2e names, and it records every request it answers.
//
// The control routes are for the e2e only and listen on loopback:
//
//	PUT /control  {"refuse_user_id": "<chit user ID>", "fail": false}
//	              refuse_user_id is refused with reason e2e_refused;
//	              fail makes every check answer 500.
//	GET /control/calls  the requests answered, oldest first.
//	GET /healthz        200 once the Hydra client is registered.
//
// It is test code: never run it beside a real chitd.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Bounds.
const (
	maxCalls     = 1000
	maxBodyBytes = 64 << 10
)

type checkRequest struct {
	ChannelID        string `json:"channel_id"`
	UserID           string `json:"user_id"`
	KratosIdentityID string `json:"kratos_identity_id"`
	ActorType        string `json:"actor_type"`
	AddedBy          string `json:"added_by"`
	ViaActorID       string `json:"via_actor_id"`
}

type control struct {
	RefuseUserID string `json:"refuse_user_id"`
	Fail         bool   `json:"fail"`
}

type stub struct {
	hydraAdmin   string
	clientID     string
	clientSecret string
	http         *http.Client
	ready        atomic.Bool

	mu    sync.Mutex
	ctl   control
	calls []checkRequest
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	s := &stub{
		hydraAdmin:   strings.TrimRight(env("HOOKSTUB_HYDRA_ADMIN_URL", "http://127.0.0.1:4445"), "/"),
		clientID:     env("HOOKSTUB_CLIENT_ID", "chit-e2e-memberhook"),
		clientSecret: env("HOOKSTUB_CLIENT_SECRET", "chit-e2e-memberhook-secret"),
		http:         &http.Client{Timeout: 5 * time.Second},
		calls:        []checkRequest{},
	}
	go s.registerClient(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("POST /membership-check", s.check)
	mux.HandleFunc("PUT /control", s.setControl)
	mux.HandleFunc("GET /control/calls", s.listCalls)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		if !s.ready.Load() {
			http.Error(w, "the Hydra client is not registered yet", http.StatusServiceUnavailable)
			return
		}
		write(w, []byte("ok\n"))
	})
	addr := env("HOOKSTUB_LISTEN", "127.0.0.1:4490")
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	slog.Info("hookstub listening", "addr", addr, "client_id", s.clientID)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("hookstub stopped", "error", err)
		os.Exit(1)
	}
}

// registerClient creates chitd's client in Hydra, or resets its secret when
// it exists, retrying until Hydra answers.
func (s *stub) registerClient(ctx context.Context) {
	body, err := json.Marshal(map[string]any{
		"client_id": s.clientID, "client_secret": s.clientSecret,
		"grant_types": []string{"client_credentials"}, "token_endpoint_auth_method": "client_secret_basic",
	})
	if err != nil {
		// Not reachable for a map of strings; /healthz stays 503.
		slog.Error("encode the hydra client", "error", err)
		return
	}
	for {
		err := s.putClient(ctx, body)
		if err == nil {
			s.ready.Store(true)
			slog.Info("hydra client registered", "client_id", s.clientID)
			return
		}
		slog.Warn("hydra not ready; retrying", "error", err)
		time.Sleep(2 * time.Second)
	}
}

func (s *stub) putClient(ctx context.Context, body []byte) error {
	status, err := s.send(ctx, http.MethodPost, s.hydraAdmin+"/admin/clients", body)
	if err != nil {
		return err
	}
	if status == http.StatusConflict {
		status, err = s.send(ctx, http.MethodPut, s.hydraAdmin+"/admin/clients/"+url.PathEscape(s.clientID), body)
		if err != nil {
			return err
		}
	}
	if status != http.StatusCreated && status != http.StatusOK {
		return fmt.Errorf("hydra answered %d", status)
	}
	return nil
}

func (s *stub) send(ctx context.Context, method, target string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return 0, err
	}
	if err := resp.Body.Close(); err != nil {
		slog.Debug("close the hydra response body", "error", err)
	}
	return resp.StatusCode, nil
}

// introspect reports whether token is an active token of chitd's client.
func (s *stub) introspect(ctx context.Context, token string) (bool, error) {
	form := url.Values{"token": {token}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.hydraAdmin+"/admin/oauth2/introspect", strings.NewReader(form))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.http.Do(req)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Debug("close the introspection response body", "error", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("introspection answered %d", resp.StatusCode)
	}
	var got struct {
		Active   bool   `json:"active"`
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		return false, err
	}
	return got.Active && got.ClientID == s.clientID, nil
}

func (s *stub) check(w http.ResponseWriter, r *http.Request) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token == "" {
		http.Error(w, "no bearer token", http.StatusUnauthorized)
		return
	}
	valid, err := s.introspect(r.Context(), token)
	if err != nil {
		slog.Warn("introspection failed", "error", err)
		http.Error(w, "introspection failed", http.StatusBadGateway)
		return
	}
	if !valid {
		http.Error(w, "the token is not an active token of "+s.clientID, http.StatusUnauthorized)
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "want application/json", http.StatusUnsupportedMediaType)
		return
	}
	var req checkRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "bad request body", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if len(s.calls) < maxCalls {
		s.calls = append(s.calls, req)
	}
	ctl := s.ctl
	s.mu.Unlock()
	slog.Info("membership check", "channel_id", req.ChannelID, "user_id", req.UserID,
		"actor_type", req.ActorType, "added_by", req.AddedBy, "via_actor_id", req.ViaActorID)

	w.Header().Set("Content-Type", "application/json")
	switch {
	case ctl.Fail:
		w.WriteHeader(http.StatusInternalServerError)
		write(w, []byte(`{"error":"the e2e asked for a failure"}`))
	case ctl.RefuseUserID != "" && req.UserID == ctl.RefuseUserID:
		encode(w, map[string]any{
			"allow": false, "reason": "e2e_refused", "detail": "The e2e stub hook refuses this user.",
		})
	default:
		write(w, []byte(`{"allow":true}`))
	}
}

// write sends b; a failure means the client left, so it is only logged.
func write(w http.ResponseWriter, b []byte) {
	if _, err := w.Write(b); err != nil {
		slog.Debug("write the response", "error", err)
	}
}

// encode sends v as JSON; a failure means the client left, so it is only
// logged.
func encode(w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Debug("encode the response", "error", err)
	}
}

func (s *stub) setControl(w http.ResponseWriter, r *http.Request) {
	var ctl control
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&ctl); err != nil {
		http.Error(w, "bad request body", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.ctl = ctl
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *stub) listCalls(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	calls := make([]checkRequest, len(s.calls))
	copy(calls, s.calls)
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	encode(w, calls)
}
