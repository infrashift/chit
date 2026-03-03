package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/infrashift/chit/internal/model"
)

// NewValidationMiddleware parses the OpenAPI spec and returns a chi-compatible
// middleware that validates incoming requests against it. Authentication
// validation is skipped because Oathkeeper handles auth externally.
func NewValidationMiddleware(specBytes []byte) (func(http.Handler) http.Handler, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(specBytes)
	if err != nil {
		return nil, fmt.Errorf("load openapi spec: %w", err)
	}

	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("validate openapi spec: %w", err)
	}

	slog.Info("openapi validation middleware initialized", "paths", len(doc.Paths.Map()))

	mw := nethttpmiddleware.OapiRequestValidatorWithOptions(doc, &nethttpmiddleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
		ErrorHandler: validationErrorHandler,
	})

	return mw, nil
}

func validationErrorHandler(w http.ResponseWriter, message string, statusCode int) {
	WriteError(w, model.NewAppError(
		"OpenAPIValidation",
		message,
		"",
		statusCode,
	))
}
