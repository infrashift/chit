package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/config"
)

// httpKetoWriter implements command.KetoWriter via the Keto HTTP write API.
type httpKetoWriter struct {
	writeURL string
	client   *http.Client
}

// WriteSubjectSetRelation writes a tuple whose subject is another relation.
// Keto expands a subject set when checking; a subject_id holding the same
// "Role:admin#member" text is an opaque string and matches nobody, which is
// why role-based command grants have to be written this way.
func (h *httpKetoWriter) WriteSubjectSetRelation(ctx context.Context, namespace, object, relation,
	subjectNamespace, subjectObject, subjectRelation string) error {
	url := fmt.Sprintf("%s/admin/relation-tuples", h.writeURL)

	body := map[string]any{
		"namespace": namespace,
		"object":    object,
		"relation":  relation,
		"subject_set": map[string]string{
			"namespace": subjectNamespace,
			"object":    subjectObject,
			"relation":  subjectRelation,
		},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal keto subject-set write: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create keto request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("keto subject-set write: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("keto subject-set write returned %d", resp.StatusCode)
	}
	return nil
}

func (h *httpKetoWriter) WriteRelation(ctx context.Context, namespace, object, relation, subjectID string) error {
	url := fmt.Sprintf("%s/admin/relation-tuples", h.writeURL)

	body := map[string]string{
		"namespace":  namespace,
		"object":     object,
		"relation":   relation,
		"subject_id": subjectID,
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal keto write: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create keto request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("keto write: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("keto write: status %d", resp.StatusCode)
	}
	return nil
}

func (h *httpKetoWriter) DeleteRelation(ctx context.Context, namespace, object, relation, subjectID string) error {
	url := fmt.Sprintf("%s/admin/relation-tuples?namespace=%s&object=%s&relation=%s&subject_id=%s",
		h.writeURL, namespace, object, relation, subjectID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create keto delete request: %w", err)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("keto delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("keto delete: status %d", resp.StatusCode)
	}
	return nil
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	cueDir := cfg.CommandsCUEDir
	if cueDir == "" {
		cueDir = "auth"
	}

	slog.Info("loading CUE definitions", "dir", cueDir)
	cueCfg, err := command.LoadCUE(cueDir)
	if err != nil {
		log.Fatalf("failed to load CUE: %v", err)
	}

	slog.Info("CUE loaded",
		"commands", len(cueCfg.Commands),
		"roles", len(cueCfg.Roles),
		"actors", len(cueCfg.Actors))

	keto := &httpKetoWriter{
		writeURL: cfg.KetoWriteURL,
		client:   &http.Client{Timeout: 10 * time.Second},
	}

	rec := command.NewReconciler(keto)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := rec.Reconcile(ctx, cueCfg); err != nil {
		log.Fatalf("reconcile failed: %v", err)
	}

	slog.Info("reconciliation complete")
}
