package api

import "net/http"

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
	if base == nil {
		base = http.DefaultTransport
	}
	return &authTransport{base: base, tokens: tokens, headerName: headerName}
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid mutating the original.
	r := req.Clone(req.Context())
	r.Header.Set(t.headerName, t.tokens.Get())
	return t.base.RoundTrip(r)
}
