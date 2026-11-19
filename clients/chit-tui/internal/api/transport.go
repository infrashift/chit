package api

import (
	"net/http"

	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
)

// TokenSource supplies the session token for each request. It is read on
// every request, so a token that changes after sign-in is picked up without
// rebuilding the client. *auth.TokenStore satisfies it.
type TokenSource interface {
	Get() string
}

// staticToken is a TokenSource whose token never changes.
type staticToken string

// Get returns the token.
func (s staticToken) Get() string { return string(s) }

// authTransport injects an auth header into every request.
type authTransport struct {
	base       http.RoundTripper
	tokens     TokenSource
	headerName string
}

func newAuthTransport(base http.RoundTripper, tokens TokenSource) *authTransport {
	return newAuthTransportWithHeader(base, tokens, "X-Session-Token")
}

func newAuthTransportWithHeader(base http.RoundTripper, tokens TokenSource, headerName string) *authTransport {
	assert.That(tokens != nil, "newAuthTransportWithHeader: nil token source")
	assert.That(headerName != "", "newAuthTransportWithHeader: empty header name")
	if base == nil {
		base = http.DefaultTransport
	}
	assert.That(base != nil, "newAuthTransportWithHeader: no base transport")
	return &authTransport{base: base, tokens: tokens, headerName: headerName}
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	assert.That(req != nil, "authTransport.RoundTrip: nil request")
	assert.That(t.base != nil && t.tokens != nil, "authTransport.RoundTrip: transport built without a base or a token source")
	assert.That(t.headerName != "", "authTransport.RoundTrip: empty header name")
	// Clone the request to avoid mutating the original.
	r := req.Clone(req.Context())
	token := t.tokens.Get()
	r.Header.Set(t.headerName, token)
	assert.That(r.Header.Get(t.headerName) == token, "authTransport.RoundTrip: auth header not set")
	assert.That(r != req, "authTransport.RoundTrip: the original request must not be changed")
	return t.base.RoundTrip(r)
}
