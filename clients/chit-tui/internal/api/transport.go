package api

import "net/http"

// authTransport injects an auth header into every request.
type authTransport struct {
	base       http.RoundTripper
	tokenFn    func() string
	headerName string
}

func newAuthTransport(base http.RoundTripper, tokenFn func() string) *authTransport {
	return newAuthTransportWithHeader(base, tokenFn, "X-Session-Token")
}

func newAuthTransportWithHeader(base http.RoundTripper, tokenFn func() string, headerName string) *authTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &authTransport{base: base, tokenFn: tokenFn, headerName: headerName}
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid mutating the original.
	r := req.Clone(req.Context())
	r.Header.Set(t.headerName, t.tokenFn())
	return t.base.RoundTrip(r)
}
