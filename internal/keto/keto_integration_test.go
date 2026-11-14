//go:build integration

package keto_test

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/infrashift/chit/internal/command"
	"github.com/infrashift/chit/internal/keto"
	"github.com/infrashift/chit/internal/model"
)

// These tests run the Keto client and the reconciler against a real Keto
// server, so they pin down the authorization behavior Chit depends on, not
// only the requests it sends. `make test-keto` starts a throwaway server with
// Chit's schema and runs them; point KETO_IMAGE at another build to compare
// two Keto versions with the same assertions.
//
// They write to chit/command, which the reconciler owns: never point them at
// a Keto you care about.

func testClient(t *testing.T) *keto.Client {
	t.Helper()
	read, write := os.Getenv("CHIT_KETO_READ_URL"), os.Getenv("CHIT_KETO_WRITE_URL")
	if read == "" || write == "" {
		t.Fatal("CHIT_KETO_READ_URL and CHIT_KETO_WRITE_URL must be set")
	}
	return keto.New(read, write, &http.Client{Timeout: 10 * time.Second})
}

// uniq makes object and subject IDs that no other run of the tests uses.
func uniq(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// assertCheck asserts the check result. Keto's /relation-tuples/check answers
// a denied check with 403, which the client reports as an error; a caller
// such as InterceptSlashCommand denies on any error, so "denied" here is
// "not allowed, with an error", the same with Keto v0.12 and keto-cue.
func assertCheck(t *testing.T, c *keto.Client, ns, object, relation, subject string, want bool) {
	t.Helper()
	got, err := c.Check(t.Context(), ns, object, relation, subject)
	if want {
		if err != nil || !got {
			t.Errorf("Check(%s:%s#%s@%s) = %v, %v; want allowed", ns, object, relation, subject, got, err)
		}
		return
	}
	if got {
		t.Errorf("Check(%s:%s#%s@%s) = allowed; want denied", ns, object, relation, subject)
	}
	if err == nil || !strings.Contains(err.Error(), "status 403") {
		t.Errorf("Check(%s:%s#%s@%s) error = %v; want status 403", ns, object, relation, subject, err)
	}
}

func TestKetoIntegration_ChannelMembership(t *testing.T) {
	c := testClient(t)
	ns := model.KetoNamespaceChannel
	channel, alice, bob := uniq("channel"), uniq("alice"), uniq("bob")

	if err := c.WriteRelation(t.Context(), ns, channel, model.KetoRelationMember, alice); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Writing the same tuple twice is not an error: chitd re-mirrors members.
	if err := c.WriteRelation(t.Context(), ns, channel, model.KetoRelationMember, alice); err != nil {
		t.Fatalf("second write: %v", err)
	}
	assertCheck(t, c, ns, channel, model.KetoRelationMember, alice, true)
	assertCheck(t, c, ns, channel, model.KetoRelationMember, bob, false)

	if err := c.DeleteRelation(t.Context(), ns, channel, model.KetoRelationMember, alice); err != nil {
		t.Fatalf("delete: %v", err)
	}
	assertCheck(t, c, ns, channel, model.KetoRelationMember, alice, false)
	// Deleting a tuple that is gone is not an error either.
	if err := c.DeleteRelation(t.Context(), ns, channel, model.KetoRelationMember, alice); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}

// TestKetoIntegration_ListPaginates writes more tuples than one list page
// holds and expects ListRelations to return all of them.
func TestKetoIntegration_ListPaginates(t *testing.T) {
	c := testClient(t)
	ns := model.KetoNamespaceChannel
	channel := uniq("paged")
	const n = 520 // more than listPageSize (500)

	for i := range n {
		if err := c.WriteRelation(t.Context(), ns, channel, model.KetoRelationMember, fmt.Sprintf("user-%03d", i)); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	t.Cleanup(func() {
		for i := range n {
			_ = c.DeleteRelation(t.Context(), ns, channel, model.KetoRelationMember, fmt.Sprintf("user-%03d", i))
		}
	})

	all, err := c.ListRelations(t.Context(), ns)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := 0
	for _, tu := range all {
		if tu.Object == channel {
			got++
		}
	}
	if got != n {
		t.Fatalf("listed %d tuples for %s, want %d", got, channel, n)
	}
}

// TestKetoIntegration_ReconcileCommands runs the reconciler as
// chit-reconcile does and checks the grants that result, then takes a command
// from a role and a role from an actor and checks that both are revoked.
func TestKetoIntegration_ReconcileCommands(t *testing.T) {
	c := testClient(t)
	ns := model.KetoNamespaceCommand
	rec := command.NewReconciler(c)
	admin, user, stranger := uniq("admin"), uniq("user"), uniq("stranger")
	exec := func(cmd, actor string, want bool) {
		t.Helper()
		assertCheck(t, c, ns, "Command:"+cmd, model.KetoRelationExecute, "Actor:"+actor, want)
	}

	cfg := &command.CUEConfig{
		Roles: []command.CUERole{
			{Name: "admin", AllowedCommands: []string{"help", "kick"}},
			{Name: "user", AllowedCommands: []string{"help"}},
		},
		Actors: []command.CUEActor{
			{ID: admin, Roles: []string{"admin"}},
			{ID: user, Roles: []string{"user"}},
		},
	}
	if err := rec.Reconcile(t.Context(), cfg); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// A command reaches an actor only through a role's member subject set.
	exec("help", admin, true)
	exec("kick", admin, true)
	exec("help", user, true)
	exec("kick", user, false)
	exec("help", stranger, false)
	exec("topic", admin, false)

	want := []string{
		ns + ":Command:help#execute@" + ns + ":Role:admin#member",
		ns + ":Command:help#execute@" + ns + ":Role:user#member",
		ns + ":Command:kick#execute@" + ns + ":Role:admin#member",
		ns + ":Role:admin#member@Actor:" + admin,
		ns + ":Role:user#member@Actor:" + user,
	}
	assertTuples(t, c, ns, want)

	// Revoke: admin loses kick, and user loses its role.
	cfg.Roles[0].AllowedCommands = []string{"help"}
	cfg.Actors = cfg.Actors[:1]
	if err := rec.Reconcile(t.Context(), cfg); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	exec("help", admin, true)
	exec("kick", admin, false)
	exec("help", user, false)
	assertTuples(t, c, ns, []string{
		ns + ":Command:help#execute@" + ns + ":Role:admin#member",
		ns + ":Command:help#execute@" + ns + ":Role:user#member",
		ns + ":Role:admin#member@Actor:" + admin,
	})

	// An empty configuration revokes everything the reconciler wrote.
	if err := rec.Reconcile(t.Context(), &command.CUEConfig{}); err != nil {
		t.Fatalf("empty reconcile: %v", err)
	}
	exec("help", admin, false)
	assertTuples(t, c, ns, nil)
}

func assertTuples(t *testing.T, c *keto.Client, ns string, want []string) {
	t.Helper()
	tuples, err := c.ListRelations(t.Context(), ns)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// A set: Keto stores a tuple once per write, so a tuple the reconciler
	// writes on every run is listed once per run. Checks are unaffected, and
	// a delete removes every copy.
	seen := map[string]bool{}
	got := []string{}
	for i := range tuples {
		if s := tuples[i].String(); !seen[s] {
			seen[s] = true
			got = append(got, s)
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("tuples in %s:\n%s\nwant:\n%s", ns, strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
