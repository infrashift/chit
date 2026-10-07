// Package api is the HTTP client for the Chit REST API. Every request
// carries the session token in a configurable header and gives up after
// DefaultTimeout. Failures come back as *APIError, and IsUnauthorized
// recognizes an expired session whichever request noticed it.
package api
