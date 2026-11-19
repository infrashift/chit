package blob

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/infrashift/chit/internal/assert"
	"github.com/infrashift/chit/internal/errlog"
)

// S3Config names a bucket on an S3-compatible endpoint.
type S3Config struct {
	// Endpoint is the base URL, such as http://localhost:8333. Requests are
	// path-style: <Endpoint>/<Bucket>/<key>.
	Endpoint        string
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	// Client defaults to one with a 30-second timeout.
	Client *http.Client
}

// S3 is a Store on an S3-compatible bucket.
type S3 struct {
	base   *url.URL
	bucket string
	region string
	key    string
	secret string
	client *http.Client
	clock  clock
}

// clock gives the signing time; tests may use a fixed one.
type clock interface {
	Now() time.Time
}

// systemClock is the production clock.
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// NewS3 checks cfg and returns the client. It does not contact the
// endpoint; EnsureBucket does.
func NewS3(cfg *S3Config) (*S3, error) {
	assert.That(cfg != nil, "NewS3: nil config")
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("blob: S3 endpoint %q is not an http(s) URL", cfg.Endpoint)
	}
	if cfg.Bucket == "" || cfg.Region == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, errors.New("blob: S3 needs a bucket, a region, an access key ID and a secret access key")
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	s := &S3{base: u, bucket: cfg.Bucket, region: cfg.Region, key: cfg.AccessKeyID,
		secret: cfg.SecretAccessKey, client: client, clock: systemClock{}}
	assert.That(s.client != nil && s.base != nil, "NewS3: nil client or base URL")
	assert.That(!strings.HasSuffix(s.base.Path, "/"), "NewS3: the base path ends in a slash")
	return s, nil
}

func (s *S3) objectURL(key string) string {
	assert.That(s.base != nil, "S3.objectURL: nil base URL")
	assert.That(s.bucket != "", "S3.objectURL: empty bucket")
	u := *s.base
	u.Path = s.base.Path + "/" + s.bucket
	if key != "" {
		u.Path += "/" + key
	}
	assert.That(strings.HasPrefix(u.Path, s.base.Path+"/"+s.bucket), "S3.objectURL: the path is outside the bucket")
	return u.String()
}

// do signs and sends one request. body may be nil.
func (s *S3) do(ctx context.Context, method, key string, body []byte, header http.Header) (*http.Response, error) {
	assert.That(ctx != nil, "S3.do: nil context")
	assert.That(method != "", "S3.do: empty method")
	assert.That(s.client != nil && s.clock != nil, "S3.do: not made by NewS3")
	var rd io.Reader = http.NoBody
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.objectURL(key), rd)
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	if body != nil {
		req.ContentLength = int64(len(body))
	}
	sign(req, body, s.key, s.secret, s.region, s.clock.Now().UTC())
	assert.That(req.Header.Get("Authorization") != "", "S3.do: the request is not signed")
	assert.That(body == nil || req.ContentLength == int64(len(body)), "S3.do: wrong content length")
	return s.client.Do(req)
}

// failure turns a non-2xx response into an error with the start of its body.
func failure(op string, resp *http.Response) error {
	assert.That(op != "", "failure: empty operation")
	assert.That(resp != nil && resp.Body != nil, "failure: nil response or body")
	assert.That(resp.StatusCode != http.StatusOK, "failure: called for HTTP 200")
	b, err := io.ReadAll(io.LimitReader(resp.Body, 512))
	errlog.Debug(err, "read the body of an S3 error response")
	assert.That(len(b) <= 512, "failure: read more than the limit")
	return fmt.Errorf("blob: S3 %s: HTTP %d: %s", op, resp.StatusCode, strings.TrimSpace(string(b)))
}

func (s *S3) Put(ctx context.Context, key, contentType string, data []byte) error {
	resp, err := s.do(ctx, http.MethodPut, key, data, http.Header{"Content-Type": {contentType}})
	if err != nil {
		return fmt.Errorf("blob: S3 put: %w", err)
	}
	assert.That(resp.Body != nil, "S3.Put: nil response body")
	defer func() { errlog.Debug(resp.Body.Close(), "close S3 put response body") }()
	if resp.StatusCode/100 != 2 {
		return failure("put", resp)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		errlog.Debug(err, "drain the S3 put response body")
	}
	return nil
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	resp, err := s.do(ctx, http.MethodGet, key, nil, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("blob: S3 get: %w", err)
	}
	assert.That(resp.Body != nil, "S3.Get: nil response body")
	if resp.StatusCode == http.StatusNotFound {
		errlog.Debug(resp.Body.Close(), "close S3 get response body")
		return nil, 0, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		defer func() { errlog.Debug(resp.Body.Close(), "close S3 get response body") }()
		return nil, 0, failure("get", resp)
	}
	return resp.Body, resp.ContentLength, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	resp, err := s.do(ctx, http.MethodDelete, key, nil, nil)
	if err != nil {
		return fmt.Errorf("blob: S3 delete: %w", err)
	}
	assert.That(resp.Body != nil, "S3.Delete: nil response body")
	defer func() { errlog.Debug(resp.Body.Close(), "close S3 delete response body") }()
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNotFound {
		return failure("delete", resp)
	}
	return nil
}

// EnsureBucket creates the bucket unless it exists.
func (s *S3) EnsureBucket(ctx context.Context) error {
	assert.That(s.bucket != "", "S3.EnsureBucket: empty bucket")
	resp, err := s.do(ctx, http.MethodHead, "", nil, nil)
	if err != nil {
		return fmt.Errorf("blob: S3 head bucket: %w", err)
	}
	assert.That(resp.Body != nil, "S3.EnsureBucket: nil head response body")
	errlog.Debug(resp.Body.Close(), "close S3 head bucket response body")
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
	default:
		return fmt.Errorf("blob: S3 head bucket %s: HTTP %d", s.bucket, resp.StatusCode)
	}
	resp, err = s.do(ctx, http.MethodPut, "", nil, nil)
	if err != nil {
		return fmt.Errorf("blob: S3 create bucket: %w", err)
	}
	defer func() { errlog.Debug(resp.Body.Close(), "close S3 create bucket response body") }()
	// Another chitd may have created it in between.
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusConflict {
		return failure("create bucket", resp)
	}
	return nil
}

// ─── Signature Version 4 ─────────────────────────────────────────

const (
	sigAlgorithm = "AWS4-HMAC-SHA256"
	sigService   = "s3"
	amzDate      = "20060102T150405Z"
)

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	out := hex.EncodeToString(sum[:])
	assert.That(len(out) == 2*sha256.Size, "sha256Hex: wrong length")
	return out
}

func hmacSHA256(key []byte, data string) []byte {
	assert.That(len(key) > 0, "hmacSHA256: empty key")
	h := hmac.New(sha256.New, key)
	_, err := h.Write([]byte(data))
	assert.That(err == nil, "hmacSHA256: hash.Hash.Write never returns an error")
	sum := h.Sum(nil)
	assert.That(len(sum) == sha256.Size, "hmacSHA256: wrong length")
	return sum
}

// sign adds x-amz-date, x-amz-content-sha256 and Authorization to req. It
// signs the host and every header already on req.
func sign(req *http.Request, body []byte, accessKey, secret, region string, now time.Time) {
	assert.That(req != nil && req.URL != nil, "sign: nil request or URL")
	assert.That(accessKey != "" && secret != "" && region != "", "sign: empty credentials or region")
	stamp := now.Format(amzDate)
	assert.That(len(stamp) >= 8, "sign: short time stamp")
	date := stamp[:8]
	req.Header.Set("X-Amz-Date", stamp)
	req.Header.Set("X-Amz-Content-Sha256", sha256Hex(body))

	canonHeaders, signed := canonicalHeaders(req)
	canonical := strings.Join([]string{
		req.Method,
		uriEncodePath(req.URL.EscapedPath()),
		canonicalQuery(req.URL.Query()),
		canonHeaders,
		signed,
		sha256Hex(body),
	}, "\n")
	scope := date + "/" + region + "/" + sigService + "/aws4_request"
	toSign := sigAlgorithm + "\n" + stamp + "\n" + scope + "\n" + sha256Hex([]byte(canonical))
	signature := hex.EncodeToString(hmacSHA256(signingKey(secret, date, region), toSign))
	assert.That(len(signature) == 2*sha256.Size, "sign: wrong signature length")

	req.Header.Set("Authorization", sigAlgorithm+" Credential="+accessKey+"/"+scope+
		", SignedHeaders="+signed+", Signature="+signature)
}

// canonicalHeaders returns the canonical header block of req (host and
// every header, lowercased and sorted) and the signed header list.
func canonicalHeaders(req *http.Request) (block, signed string) {
	assert.That(req != nil && req.URL != nil, "canonicalHeaders: nil request or URL")
	headers := map[string]string{"host": req.URL.Host}
	for k, v := range req.Header {
		headers[strings.ToLower(k)] = strings.Join(v, ",")
	}
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	slices.Sort(names)
	assert.That(len(names) == len(headers), "canonicalHeaders: a header name is missing")
	var canon strings.Builder
	for _, k := range names {
		canon.WriteString(k + ":" + strings.TrimSpace(headers[k]) + "\n")
	}
	assert.That(slices.Contains(names, "host"), "canonicalHeaders: host is not signed")
	return canon.String(), strings.Join(names, ";")
}

// signingKey derives the SigV4 signing key for one day, region and S3.
func signingKey(secret, date, region string) []byte {
	assert.That(len(date) == 8, "signingKey: date is not YYYYMMDD")
	assert.That(region != "", "signingKey: empty region")
	key := hmacSHA256([]byte("AWS4"+secret), date)
	key = hmacSHA256(key, region)
	key = hmacSHA256(key, sigService)
	key = hmacSHA256(key, "aws4_request")
	assert.That(len(key) == sha256.Size, "signingKey: wrong key length")
	return key
}

// uriEncodePath re-encodes an escaped path the way SigV4 wants it for S3:
// each byte other than unreserved characters and "/" percent-encoded once.
func uriEncodePath(escaped string) string {
	raw, err := url.PathUnescape(escaped)
	if err != nil {
		raw = escaped
	}
	if raw == "" {
		return "/"
	}
	out := uriEncode(raw, true)
	assert.That(len(out) >= len(raw), "uriEncodePath: the encoding is shorter than the path")
	return out
}

// uriEncode percent-encodes every byte of s but the unreserved characters,
// and "/" when keepSlash is set.
func uriEncode(s string, keepSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUnreserved(c) || (keepSlash && c == '/') {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	assert.That(b.Len() >= len(s), "uriEncode: the encoding is shorter than the input")
	assert.That(b.Len() <= 3*len(s), "uriEncode: more than three bytes for each input byte")
	return b.String()
}

func isUnreserved(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' ||
		c == '-' || c == '_' || c == '.' || c == '~'
}

func canonicalQuery(q url.Values) string {
	keys := slices.Sorted(maps.Keys(q))
	assert.That(len(keys) == len(q), "canonicalQuery: a key is missing")
	var parts []string
	for _, k := range keys {
		vals := slices.Clone(q[k])
		slices.Sort(vals)
		for _, v := range vals {
			parts = append(parts, uriEncode(k, false)+"="+uriEncode(v, false))
		}
	}
	assert.That(len(parts) >= len(keys), "canonicalQuery: a key is missing")
	return strings.Join(parts, "&")
}
