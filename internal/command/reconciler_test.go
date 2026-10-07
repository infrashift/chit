package command

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/infrashift/chit/internal/keto"
)

// fakeKeto is an in-memory Keto namespace. It records how tuples were
// written, since a role grant written as a subject_id string instead of a
// subject set is stored fine and matches nobody.
type fakeKeto struct {
	tuples      map[string]keto.Tuple
	subjectSets int
	subjectIDs  []string
	listErr     error
}

func newFakeKeto(existing ...keto.Tuple) *fakeKeto {
	f := &fakeKeto{tuples: map[string]keto.Tuple{}}
	for i := range existing {
		f.tuples[existing[i].String()] = existing[i]
	}
	return f
}

func (f *fakeKeto) WriteRelation(_ context.Context, namespace, object, relation, subjectID string) error {
	f.subjectIDs = append(f.subjectIDs, subjectID)
	t := keto.Tuple{Namespace: namespace, Object: object, Relation: relation, SubjectID: subjectID}
	f.tuples[t.String()] = t
	return nil
}

func (f *fakeKeto) WriteSubjectSetRelation(_ context.Context, namespace, object, relation,
	subjectNamespace, subjectObject, subjectRelation string) error {
	f.subjectSets++
	t := keto.Tuple{Namespace: namespace, Object: object, Relation: relation,
		SubjectSet: &keto.SubjectSet{Namespace: subjectNamespace, Object: subjectObject, Relation: subjectRelation}}
	f.tuples[t.String()] = t
	return nil
}

func (f *fakeKeto) ListRelations(_ context.Context, namespace string) ([]keto.Tuple, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []keto.Tuple
	for _, t := range f.tuples {
		if t.Namespace == namespace {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeKeto) DeleteTuple(_ context.Context, t *keto.Tuple) error {
	delete(f.tuples, t.String())
	return nil
}

func (f *fakeKeto) keys() []string {
	var ks []string
	for k := range f.tuples {
		ks = append(ks, k)
	}
	slices.Sort(ks)
	return ks
}

var adminAndUser = &CUEConfig{
	Commands: []*Command{{ID: "help", Slug: "help"}, {ID: "kick", Slug: "kick"}},
	Roles: []CUERole{
		{Name: "admin", AllowedCommands: []string{"help", "kick"}},
		{Name: "user", AllowedCommands: []string{"help"}},
	},
	Actors: []CUEActor{
		{ID: "alice-uuid", Roles: []string{"admin"}},
		{ID: "bob-uuid", Roles: []string{"user"}},
	},
}

func TestReconcile(t *testing.T) {
	k := newFakeKeto()
	if err := NewReconciler(k).Reconcile(context.Background(), adminAndUser); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Role grants are subject sets (note the namespaced right-hand side) so
	// Keto expands them to the actors bound in the second group.
	want := []string{
		"chit/command:Command:help#execute@chit/command:Role:admin#member",
		"chit/command:Command:help#execute@chit/command:Role:user#member",
		"chit/command:Command:kick#execute@chit/command:Role:admin#member",
		"chit/command:Role:admin#member@Actor:alice-uuid",
		"chit/command:Role:user#member@Actor:bob-uuid",
	}
	if got := k.keys(); !slices.Equal(got, want) {
		t.Fatalf("tuples:\n got %v\nwant %v", got, want)
	}
}

// Reconcile used to only write, so a command taken away from a role, or a
// role taken away from an actor, in the CUE stayed granted in Keto.
func TestReconcileRevokesWhatIsNoLongerDeclared(t *testing.T) {
	k := newFakeKeto()
	if err := NewReconciler(k).Reconcile(context.Background(), adminAndUser); err != nil {
		t.Fatal(err)
	}

	narrowed := &CUEConfig{
		Commands: adminAndUser.Commands,
		Roles: []CUERole{
			{Name: "admin", AllowedCommands: []string{"help"}}, // kick taken away
			{Name: "user", AllowedCommands: []string{"help"}},
		},
		Actors: []CUEActor{{ID: "alice-uuid", Roles: []string{"admin"}}}, // bob removed
	}
	if err := NewReconciler(k).Reconcile(context.Background(), narrowed); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	want := []string{
		"chit/command:Command:help#execute@chit/command:Role:admin#member",
		"chit/command:Command:help#execute@chit/command:Role:user#member",
		"chit/command:Role:admin#member@Actor:alice-uuid",
	}
	if got := k.keys(); !slices.Equal(got, want) {
		t.Fatalf("tuples:\n got %v\nwant %v", got, want)
	}
}

// Tuples in other namespaces (channel membership) are not the reconciler's.
func TestReconcileLeavesOtherNamespacesAlone(t *testing.T) {
	channel := keto.Tuple{Namespace: "chit/channel", Object: "c1", Relation: "member", SubjectID: "u1"}
	k := newFakeKeto(channel)
	if err := NewReconciler(k).Reconcile(context.Background(), adminAndUser); err != nil {
		t.Fatal(err)
	}
	if _, ok := k.tuples[channel.String()]; !ok {
		t.Fatal("a channel tuple was deleted")
	}
}

// If the current state cannot be read, nothing is revoked.
func TestReconcileDeletesNothingWhenListingFails(t *testing.T) {
	stale := keto.Tuple{Namespace: "chit/command", Object: "Role:old", Relation: "member", SubjectID: "Actor:x"}
	k := newFakeKeto(stale)
	k.listErr = errors.New("keto read API unreachable")

	if err := NewReconciler(k).Reconcile(context.Background(), adminAndUser); err == nil {
		t.Fatal("expected the listing error")
	}
	if _, ok := k.tuples[stale.String()]; !ok {
		t.Fatal("a tuple was revoked without the current state having been read")
	}
}

func TestReconcileNoActors(t *testing.T) {
	k := newFakeKeto()
	cfg := &CUEConfig{
		Commands: []*Command{{ID: "help", Slug: "help"}},
		Roles:    []CUERole{{Name: "user", AllowedCommands: []string{"help"}}},
	}
	if err := NewReconciler(k).Reconcile(context.Background(), cfg); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(k.tuples) != 1 {
		t.Fatalf("expected 1 tuple, got %v", k.keys())
	}
}

// The bug this guards: role grants were written with the subject set rendered
// into a subject_id string ("Role:admin#member"). Keto stores that as an
// opaque literal and never expands it, so every command check returned false
// and no slash command could be authorized for anybody.
func TestReconcileWritesRoleGrantsAsSubjectSets(t *testing.T) {
	k := newFakeKeto()
	cfg := &CUEConfig{
		Commands: []*Command{{ID: "invite", Slug: "invite"}},
		Roles:    []CUERole{{Name: "admin", AllowedCommands: []string{"invite"}}},
		Actors:   []CUEActor{{ID: "alice-uuid", Roles: []string{"admin"}}},
	}
	if err := NewReconciler(k).Reconcile(context.Background(), cfg); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if k.subjectSets != 1 {
		t.Errorf("role grants written as subject sets = %d, want 1", k.subjectSets)
	}
	for _, id := range k.subjectIDs {
		if strings.Contains(id, "#") {
			t.Errorf("subject_id %q encodes a subject set; Keto will not expand it", id)
		}
	}
}
