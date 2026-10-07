// Package keto is a small client for the Ory Keto relation-tuple API: the
// permission check on the read API, and tuple writes and deletes on the
// admin API. It depends on the standard library only, so the chit-reconcile
// binary can use it without linking any server-side dependency.
package keto

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client talks to Keto's read and write (admin) APIs.
type Client struct {
	readURL  string
	writeURL string
	http     *http.Client
}

// New returns a client for the Keto read and write APIs at the given base
// URLs. Either may be empty when the caller only uses the other.
func New(readURL, writeURL string, httpClient *http.Client) *Client {
	return &Client{readURL: readURL, writeURL: writeURL, http: httpClient}
}

// Check reports whether subjectID has relation on namespace:object.
func (c *Client) Check(ctx context.Context, namespace, object, relation, subjectID string) (bool, error) {
	var result struct {
		Allowed bool `json:"allowed"`
	}
	body := map[string]string{
		"namespace": namespace, "object": object, "relation": relation, "subject_id": subjectID,
	}
	if err := c.do(ctx, http.MethodPost, c.readURL+"/relation-tuples/check", body, &result); err != nil {
		return false, fmt.Errorf("keto check: %w", err)
	}
	return result.Allowed, nil
}

// WriteRelation writes the tuple namespace:object#relation@subjectID.
func (c *Client) WriteRelation(ctx context.Context, namespace, object, relation, subjectID string) error {
	body := map[string]string{
		"namespace": namespace, "object": object, "relation": relation, "subject_id": subjectID,
	}
	if err := c.do(ctx, http.MethodPut, c.writeURL+"/admin/relation-tuples", body, nil); err != nil {
		return fmt.Errorf("keto write: %w", err)
	}
	return nil
}

// WriteSubjectSetRelation writes a tuple whose subject is another relation.
// Keto expands a subject set when checking; a subject_id holding the same
// "Role:admin#member" text is an opaque string and matches nobody, which is
// why role-based grants have to be written this way.
func (c *Client) WriteSubjectSetRelation(ctx context.Context, namespace, object, relation,
	subjectNamespace, subjectObject, subjectRelation string) error {
	body := map[string]any{
		"namespace": namespace, "object": object, "relation": relation,
		"subject_set": map[string]string{
			"namespace": subjectNamespace, "object": subjectObject, "relation": subjectRelation,
		},
	}
	if err := c.do(ctx, http.MethodPut, c.writeURL+"/admin/relation-tuples", body, nil); err != nil {
		return fmt.Errorf("keto subject-set write: %w", err)
	}
	return nil
}

// DeleteRelation deletes the tuple namespace:object#relation@subjectID.
func (c *Client) DeleteRelation(ctx context.Context, namespace, object, relation, subjectID string) error {
	q := url.Values{
		"namespace": {namespace}, "object": {object}, "relation": {relation}, "subject_id": {subjectID},
	}
	if err := c.do(ctx, http.MethodDelete, c.writeURL+"/admin/relation-tuples?"+q.Encode(), nil, nil); err != nil {
		return fmt.Errorf("keto delete: %w", err)
	}
	return nil
}

// do sends body (when non-nil) as JSON and decodes the response into out
// (when non-nil). Any status of 300 or above is an error: a check must not
// read an error page as "not allowed", nor a failed write as done.
func (c *Client) do(ctx context.Context, method, target string, body, out any) error {
	var reader io.Reader = http.NoBody
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}
