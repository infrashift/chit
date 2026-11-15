package api

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	chitapi "github.com/infrashift/chit/api"
	"github.com/infrashift/chit/internal/model"
)

// The on-behalf-of route list (onBehalfOfRoutes), the Tag routes halod uses
// to manage a person's Tags, and the privacy of private Tags under it.

// The list in middleware.go and the operations that declare the header in
// api/openapi.yaml are the same set.
func TestOnBehalfOf_RouteListMatchesTheSpec(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]struct {
			Parameters []map[string]any `yaml:"parameters"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(chitapi.OpenAPISpec, &spec); err != nil {
		t.Fatal(err)
	}
	var declared []string
	for path, ops := range spec.Paths {
		for method, op := range ops {
			for _, p := range op.Parameters {
				if p["$ref"] == "#/components/parameters/OnBehalfOfHeader" {
					declared = append(declared, strings.ToUpper(method)+" "+normalizePath(path))
				}
			}
		}
	}
	listed := make([]string, 0, len(onBehalfOfRoutes))
	for _, route := range onBehalfOfRoutes {
		listed = append(listed, normalizePath(route.name))
	}
	slices.Sort(declared)
	slices.Sort(listed)
	if !slices.Equal(declared, listed) {
		t.Fatalf("the spec declares X-On-Behalf-Of on\n  %s\nbut middleware.go accepts it on\n  %s",
			strings.Join(declared, "\n  "), strings.Join(listed, "\n  "))
	}
}

// The whole list, by name, as HALO is told it.
func TestOnBehalfOf_TheRouteList(t *testing.T) {
	want := []string{
		"POST /posts", "POST /channels/{id}/stream_status",
		"POST /tags", "GET /tags", "POST /posts/{id}/tags", "DELETE /posts/{id}/tags/{tag_id}",
		"POST /channels/{id}/tags", "GET /channels/{id}/tags", "DELETE /channels/{id}/tags/{tag_id}",
		"POST /tags/posts", "POST /tags/channels",
		"POST /channels/{id}/files", "GET /files/{id}",
		"POST /channels", "POST /channels/{id}/members", "GET /users/me",
	}
	got := make([]string, 0, len(onBehalfOfRoutes))
	for _, route := range onBehalfOfRoutes {
		got = append(got, route.name)
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("on-behalf-of routes\n got %v\nwant %v", got, want)
	}
}

// tagRouteEnv has testUser's private Tag on the root post and on the
// channel, and bob's (extraUser's) private Tag on both too.
type tagRouteEnv struct {
	*bridgeEnv
	mine, bobs *model.Tag
}

func newTagRouteEnv(t *testing.T) *tagRouteEnv {
	t.Helper()
	e := newTagEnv(t)
	mine := e.mustCreateTag(t, "user:"+aliceKratos, "my label", model.TagScopePrivate, "", nil)
	bobs := e.mustCreateTag(t, "user:"+bobKratos, "bobs secret", model.TagScopePrivate, "", nil)
	for who, tag := range map[string]*model.Tag{aliceKratos: mine, bobKratos: bobs} {
		e.mustOK(t, http.MethodPost, "/api/v1/posts/"+testRootPost+"/tags", "user:"+who, `{"tag_id":"`+tag.ID+`"}`)
		e.mustOK(t, http.MethodPost, "/api/v1/channels/"+testChannelID+"/tags", "user:"+who, `{"tag_id":"`+tag.ID+`"}`)
	}
	return &tagRouteEnv{bridgeEnv: e, mine: mine, bobs: bobs}
}

type oboRequest struct{ method, path, body string }

func (e *tagRouteEnv) newTagRoutes() map[string]oboRequest {
	return map[string]oboRequest{
		"GET /tags":                           {http.MethodGet, "/api/v1/tags", ""},
		"GET /channels/{id}/tags":             {http.MethodGet, "/api/v1/channels/" + testChannelID + "/tags", ""},
		"POST /tags/posts":                    {http.MethodPost, "/api/v1/tags/posts", `{"post_ids":["` + testRootPost + `"]}`},
		"POST /tags/channels":                 {http.MethodPost, "/api/v1/tags/channels", `{"tag_ids":["` + e.mine.ID + `"]}`},
		"DELETE /channels/{id}/tags/{tag_id}": {http.MethodDelete, "/api/v1/channels/" + testChannelID + "/tags/" + e.mine.ID, ""},
		"DELETE /posts/{id}/tags/{tag_id}":    {http.MethodDelete, "/api/v1/posts/" + testRootPost + "/tags/" + e.mine.ID, ""},
	}
}

// Each new Tag route accepts the header from an allowlisted client, as the
// person, and refuses it from any other client.
func TestOnBehalfOf_TagRoutes(t *testing.T) {
	for name := range (&tagRouteEnv{mine: &model.Tag{}}).newTagRoutes() {
		t.Run(name, func(t *testing.T) {
			e := newTagRouteEnv(t)
			req := e.newTagRoutes()[name]
			if code, out := e.call(t, req.method, req.path, rogue, req.body, obo(testUserID)); code != http.StatusForbidden {
				t.Fatalf("a client not on the allowlist: %d %s", code, out)
			}
			code, out := e.call(t, req.method, req.path, halod, req.body, obo(testUserID))
			if code != http.StatusOK {
				t.Fatalf("the allowlisted client: %d %s", code, out)
			}
			// What the person's own Tag did shows that it ran as them.
			switch req.method {
			case http.MethodGet:
				if !strings.Contains(out, e.mine.ID) {
					t.Fatalf("the person's private tag is missing: %s", out)
				}
			case http.MethodDelete:
				_, after := e.call(t, http.MethodGet, strings.Replace(req.path, "/"+e.mine.ID, "", 1), "user:"+aliceKratos, "", nil)
				if strings.Contains(after, e.mine.ID) {
					t.Fatalf("the tag is still there: %s", after)
				}
			default:
				if !strings.Contains(out, e.mine.ID) && !strings.Contains(out, testChannelID) {
					t.Fatalf("the result does not reflect the person's tag: %s", out)
				}
			}
		})
	}
}

// Acting for a person shows that person's private Tags and nobody else's,
// by every read route; and it cannot remove another person's private Tag.
func TestOnBehalfOf_PrincipalSeesOnlyTheirOwnPrivateTags(t *testing.T) {
	e := newTagRouteEnv(t)
	reads := []oboRequest{
		{http.MethodGet, "/api/v1/tags", ""},
		{http.MethodGet, "/api/v1/tags?channel_id=" + testChannelID, ""},
		{http.MethodGet, "/api/v1/channels/" + testChannelID + "/tags", ""},
		{http.MethodPost, "/api/v1/tags/posts", `{"post_ids":["` + testRootPost + `"]}`},
	}
	for _, req := range reads {
		code, out := e.call(t, req.method, req.path, halod, req.body, obo(testUserID))
		if code != http.StatusOK || strings.Contains(out, e.bobs.ID) || strings.Contains(out, "bobs secret") ||
			!strings.Contains(out, e.mine.ID) {
			t.Errorf("%s %s for testUser: %d %s", req.method, req.path, code, out)
		}
		code, out = e.call(t, req.method, req.path, halod, req.body, obo(extraUserID))
		if code != http.StatusOK || !strings.Contains(out, e.bobs.ID) || strings.Contains(out, e.mine.ID) {
			t.Errorf("%s %s for bob: %d %s", req.method, req.path, code, out)
		}
	}

	// Filtering by bob's tag, as testUser, matches nothing.
	code, out := e.call(t, http.MethodPost, "/api/v1/tags/channels", halod, `{"tag_ids":["`+e.bobs.ID+`"]}`, obo(testUserID))
	if code != http.StatusOK || strings.TrimSpace(out) != "[]" {
		t.Errorf("filter by another person's private tag: %d %s", code, out)
	}
	// Removing bob's tag, as testUser, is the 404 of an unknown tag.
	for _, path := range []string{
		"/api/v1/posts/" + testRootPost + "/tags/" + e.bobs.ID,
		"/api/v1/channels/" + testChannelID + "/tags/" + e.bobs.ID,
	} {
		if code, body := e.call(t, http.MethodDelete, path, halod, "", obo(testUserID)); code != http.StatusNotFound {
			t.Errorf("DELETE %s for testUser: %d %s", path, code, body)
		}
	}
	_, out = e.call(t, http.MethodGet, "/api/v1/posts/"+testRootPost+"/tags", "user:"+bobKratos, "", nil)
	if !strings.Contains(out, e.bobs.ID) {
		t.Fatalf("bob's tag was removed from the post: %s", out)
	}
}

// Rename and delete of a Tag stay off the list for now.
func TestOnBehalfOf_TagRenameAndDeleteRefused(t *testing.T) {
	e := newTagRouteEnv(t)
	for _, req := range []oboRequest{
		{http.MethodPut, "/api/v1/tags/" + e.mine.ID, `{"name":"renamed"}`},
		{http.MethodDelete, "/api/v1/tags/" + e.mine.ID, ""},
		{http.MethodGet, "/api/v1/posts/" + testRootPost + "/tags", ""},
	} {
		if code, out := e.call(t, req.method, req.path, halod, req.body, obo(testUserID)); code != http.StatusBadRequest {
			t.Errorf("%s %s: %d %s, want 400", req.method, req.path, code, out)
		}
	}
}
