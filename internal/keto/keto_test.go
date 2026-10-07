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

// ListRelations follows next_page_token until Keto returns an empty one.
func TestListRelations_FollowsPages(t *testing.T) {
	var tokens []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := r.URL.Query().Get("page_token")
		tokens = append(tokens, tok)
		next, subject := "p2", "a"
		if tok == "p2" {
			next, subject = "", "b"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"relation_tuples": []Tuple{{Namespace: "ns", Object: "o", Relation: "r", SubjectID: subject}},
			"next_page_token": next,
		})
	}))
	defer srv.Close()

	got, err := New(srv.URL, "", srv.Client()).ListRelations(t.Context(), "ns")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].SubjectID != "a" || got[1].SubjectID != "b" || len(tokens) != 2 || tokens[1] != "p2" {
		t.Fatalf("got %+v after tokens %q", got, tokens)
	}
}

func TestDeleteTuple_SubjectSet(t *testing.T) {
	c, got := fakeKeto(t, http.StatusNoContent, ``)
	err := c.DeleteTuple(t.Context(), &Tuple{
		Namespace: "ns", Object: "Command:help", Relation: "execute",
		SubjectSet: &SubjectSet{Namespace: "ns", Object: "Role:admin", Relation: "member"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if q := (*got)[0].query; q != "namespace=ns&object=Command%3Ahelp&relation=execute&subject_set.namespace=ns&subject_set.object=Role%3Aadmin&subject_set.relation=member" {
		t.Fatalf("query = %s", q)
	}
}

func TestTupleString(t *testing.T) {
	id := Tuple{Namespace: "ns", Object: "Role:a", Relation: "member", SubjectID: "Actor:x"}
	set := Tuple{Namespace: "ns", Object: "Command:c", Relation: "execute",
		SubjectSet: &SubjectSet{Namespace: "ns", Object: "Role:a", Relation: "member"}}
	if id.String() != "ns:Role:a#member@Actor:x" || set.String() != "ns:Command:c#execute@ns:Role:a#member" {
		t.Fatalf("%s / %s", id.String(), set.String())
	}
}
