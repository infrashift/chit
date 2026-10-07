package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	v5emb "github.com/swaggest/swgui/v5emb"
)

// MountDocs registers the raw spec endpoint and Swagger UI.
func MountDocs(r chi.Router, specBytes []byte) {
	r.Get("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(specBytes)
	})
	r.Mount("/docs", v5emb.New("Chit API", "/api/v1/openapi.yaml", "/api/v1/docs/"))
}
