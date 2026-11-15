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
	now    func() time.Time
}

// NewS3 checks cfg and returns the client. It does not contact the
// endpoint; EnsureBucket does.
func NewS3(cfg *S3Config) (*S3, error) {
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
	return &S3{base: u, bucket: cfg.Bucket, region: cfg.Region, key: cfg.AccessKeyID,
		secret: cfg.SecretAccessKey, client: client, now: time.Now}, nil
}

func (s *S3) objectURL(key string) string {
	u := *s.base
	u.Path = s.base.Path + "/" + s.bucket
	if key != "" {
		u.Path += "/" + key
	}
	return u.String()
}

// do signs and sends one request. body may be nil.
func (s *S3) do(ctx context.Context, method, key string, body []byte, header http.Header) (*http.Response, error) {
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
	sign(req, body, s.key, s.secret, s.region, s.now().UTC())
	return s.client.Do(req)
}

// failure turns a non-2xx response into an error with the start of its body.
func failure(op string, resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return fmt.Errorf("blob: S3 %s: HTTP %d: %s", op, resp.StatusCode, strings.TrimSpace(string(b)))
}

func (s *S3) Put(ctx context.Context, key, contentType string, data []byte) error {
	resp, err := s.do(ctx, http.MethodPut, key, data, http.Header{"Content-Type": {contentType}})
	if err != nil {
		return fmt.Errorf("blob: S3 put: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return failure("put", resp)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	resp, err := s.do(ctx, http.MethodGet, key, nil, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("blob: S3 get: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		return nil, 0, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		defer func() { _ = resp.Body.Close() }()
		return nil, 0, failure("get", resp)
	}
	return resp.Body, resp.ContentLength, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	resp, err := s.do(ctx, http.MethodDelete, key, nil, nil)
	if err != nil {
		return fmt.Errorf("blob: S3 delete: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNotFound {
		return failure("delete", resp)
	}
	return nil
}

// EnsureBucket creates the bucket unless it exists.
func (s *S3) EnsureBucket(ctx context.Context) error {
	resp, err := s.do(ctx, http.MethodHead, "", nil, nil)
	if err != nil {
		return fmt.Errorf("blob: S3 head bucket: %w", err)
	}
	_ = resp.Body.Close()
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
	defer func() { _ = resp.Body.Close() }()
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
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// sign adds x-amz-date, x-amz-content-sha256 and Authorization to req. It
// signs the host and every header already on req.
func sign(req *http.Request, body []byte, accessKey, secret, region string, now time.Time) {
	stamp := now.Format(amzDate)
	date := stamp[:8]
	req.Header.Set("X-Amz-Date", stamp)
	req.Header.Set("X-Amz-Content-Sha256", sha256Hex(body))

	headers := map[string]string{"host": req.URL.Host}
	for k, v := range req.Header {
		headers[strings.ToLower(k)] = strings.Join(v, ",")
	}
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	slices.Sort(names)
	var canonHeaders strings.Builder
	for _, k := range names {
		canonHeaders.WriteString(k + ":" + strings.TrimSpace(headers[k]) + "\n")
	}
	signed := strings.Join(names, ";")

	canonical := strings.Join([]string{
		req.Method,
		uriEncodePath(req.URL.EscapedPath()),
		canonicalQuery(req.URL.Query()),
		canonHeaders.String(),
		signed,
		sha256Hex(body),
	}, "\n")
	scope := date + "/" + region + "/" + sigService + "/aws4_request"
	toSign := sigAlgorithm + "\n" + stamp + "\n" + scope + "\n" + sha256Hex([]byte(canonical))

	key := hmacSHA256([]byte("AWS4"+secret), date)
	key = hmacSHA256(key, region)
	key = hmacSHA256(key, sigService)
	key = hmacSHA256(key, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(key, toSign))

	req.Header.Set("Authorization", sigAlgorithm+" Credential="+accessKey+"/"+scope+
		", SignedHeaders="+signed+", Signature="+signature)
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
	return uriEncode(raw, true)
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
	return b.String()
}

func isUnreserved(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' ||
		c == '-' || c == '_' || c == '.' || c == '~'
}

func canonicalQuery(q url.Values) string {
	keys := slices.Sorted(maps.Keys(q))
	var parts []string
	for _, k := range keys {
		vals := slices.Clone(q[k])
		slices.Sort(vals)
		for _, v := range vals {
			parts = append(parts, uriEncode(k, false)+"="+uriEncode(v, false))
		}
	}
	return strings.Join(parts, "&")
}
