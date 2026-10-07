package keto

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type recorded struct {
	method, path, query string
	body                map[string]any
}

func fakeKeto(t *testing.T, status int, reply string) (*Client, *[]recorded) {
	t.Helper()
	var got []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := recorded{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery}
		_ = json.NewDecoder(r.Body).Decode(&rec.body)
		got = append(got, rec)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, srv.URL, srv.Client()), &got
}

func TestCheck(t *testing.T) {
	c, got := fakeKeto(t, http.StatusOK, `{"allowed":true}`)
	ok, err := c.Check(t.Context(), "ns", "obj", "rel", "sub")
	if err != nil || !ok {
		t.Fatalf("Check = %v, %v", ok, err)
	}
	r := (*got)[0]
	if r.method != http.MethodPost || r.path != "/relation-tuples/check" || r.body["subject_id"] != "sub" {
		t.Fatalf("request = %+v", r)
	}
}

// An error status must not read as "not allowed": that turns an outage into
// a wall of permission denials with no error anywhere.
func TestCheck_ErrorStatusIsAnError(t *testing.T) {
	c, _ := fakeKeto(t, http.StatusInternalServerError, `{"error":"boom"}`)
	if _, err := c.Check(t.Context(), "ns", "obj", "rel", "sub"); err == nil {
		t.Fatal("a 500 from Keto was not an error")
	}
}

func TestWriteAndDelete(t *testing.T) {
	c, got := fakeKeto(t, http.StatusOK, ``)
	if err := c.WriteRelation(t.Context(), "ns", "obj", "rel", "sub"); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteSubjectSetRelation(t.Context(), "ns", "obj", "rel", "sns", "sobj", "srel"); err != nil {
		t.Fatal(err)
	}
	// A subject with characters that are special in a query string must
	// arrive intact; the delete used to splice raw values into the URL.
	if err := c.DeleteRelation(t.Context(), "ns", "obj", "rel", "Role:admin#member&x=1"); err != nil {
		t.Fatal(err)
	}

	if (*got)[0].method != http.MethodPut || (*got)[0].body["subject_id"] != "sub" {
		t.Errorf("write = %+v", (*got)[0])
	}
	if set, _ := (*got)[1].body["subject_set"].(map[string]any); set["relation"] != "srel" {
		t.Errorf("subject-set write = %+v", (*got)[1])
	}
	del := (*got)[2]
	if del.method != http.MethodDelete || del.query != "namespace=ns&object=obj&relation=rel&subject_id=Role%3Aadmin%23member%26x%3D1" {
		t.Errorf("delete = %+v", del)
	}
}

func TestWrite_ErrorStatusIsAnError(t *testing.T) {
	c, _ := fakeKeto(t, http.StatusForbidden, ``)
	if err := c.WriteRelation(t.Context(), "ns", "obj", "rel", "sub"); err == nil {
		t.Fatal("a 403 from Keto was not an error")
	}
}
