package command

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type mockKetoWriter struct {
	tuples []string
}

func (m *mockKetoWriter) WriteRelation(_ context.Context, namespace, object, relation, subjectID string) error {
	m.tuples = append(m.tuples, fmt.Sprintf("%s:%s#%s@%s", namespace, object, relation, subjectID))
	return nil
}

// The recorded form mirrors Keto's own notation so a subject set is
// distinguishable from a literal ID in assertions.
func (m *mockKetoWriter) WriteSubjectSetRelation(_ context.Context, namespace, object, relation,
	subjectNamespace, subjectObject, subjectRelation string) error {
	m.tuples = append(m.tuples, fmt.Sprintf("%s:%s#%s@%s:%s#%s",
		namespace, object, relation, subjectNamespace, subjectObject, subjectRelation))
	return nil
}

func (m *mockKetoWriter) DeleteRelation(_ context.Context, namespace, object, relation, subjectID string) error {
	return nil
}

func TestReconcile(t *testing.T) {
	keto := &mockKetoWriter{}
	rec := NewReconciler(keto)

	cfg := &CUEConfig{
		Commands: []*Command{
			{ID: "help", Slug: "help"},
			{ID: "kick", Slug: "kick"},
		},
		Roles: []CUERole{
			{Name: "admin", AllowedCommands: []string{"help", "kick"}},
			{Name: "user", AllowedCommands: []string{"help"}},
		},
		Actors: []CUEActor{
			{ID: "alice-uuid", Roles: []string{"admin"}},
			{ID: "bob-uuid", Roles: []string{"user"}},
		},
	}

	if err := rec.Reconcile(context.Background(), cfg); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Command grants: admin gets help+kick (2), user gets help (1) = 3 tuples
	// Actor memberships: alice→admin (1), bob→user (1) = 2 tuples
	// Total: 5
	if len(keto.tuples) != 5 {
		t.Fatalf("expected 5 tuples, got %d: %v", len(keto.tuples), keto.tuples)
	}

	// Check for expected tuples.
	// Role grants must be subject sets (note the namespaced right-hand side)
	// so Keto expands them to the actors bound in the second group. Written
	// as a literal subject_id they would match nobody.
	expected := map[string]bool{
		"chit/command:Command:help#execute@chit/command:Role:admin#member": true,
		"chit/command:Command:kick#execute@chit/command:Role:admin#member": true,
		"chit/command:Command:help#execute@chit/command:Role:user#member":  true,
		"chit/command:Role:admin#member@Actor:alice-uuid":                  true,
		"chit/command:Role:user#member@Actor:bob-uuid":                     true,
	}
	for _, tuple := range keto.tuples {
		if !expected[tuple] {
			t.Errorf("unexpected tuple: %s", tuple)
		}
	}
}

func TestReconcileNoActors(t *testing.T) {
	keto := &mockKetoWriter{}
	rec := NewReconciler(keto)

	cfg := &CUEConfig{
		Commands: []*Command{{ID: "help", Slug: "help"}},
		Roles:    []CUERole{{Name: "user", AllowedCommands: []string{"help"}}},
	}

	if err := rec.Reconcile(context.Background(), cfg); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if len(keto.tuples) != 1 {
		t.Fatalf("expected 1 tuple, got %d", len(keto.tuples))
	}
}

// The bug this guards: role grants were written with the subject set rendered
// into a subject_id string ("Role:admin#member"). Keto stores that as an
// opaque literal and never expands it, so every command check returned false
// and no slash command could be authorized for anybody.
func TestReconcileWritesRoleGrantsAsSubjectSets(t *testing.T) {
	keto := &recordingKetoWriter{}
	rec := NewReconciler(keto)

	cfg := &CUEConfig{
		Commands: []*Command{{ID: "invite", Slug: "invite"}},
		Roles:    []CUERole{{Name: "admin", AllowedCommands: []string{"invite"}}},
		Actors:   []CUEActor{{ID: "alice-uuid", Roles: []string{"admin"}}},
	}

	if err := rec.Reconcile(context.Background(), cfg); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if keto.subjectSets != 1 {
		t.Errorf("role grants written as subject sets = %d, want 1", keto.subjectSets)
	}
	for _, id := range keto.subjectIDs {
		if strings.Contains(id, "#") {
			t.Errorf("subject_id %q encodes a subject set; Keto will not expand it", id)
		}
	}
}

// recordingKetoWriter distinguishes the two write forms.
type recordingKetoWriter struct {
	subjectSets int
	subjectIDs  []string
}

func (m *recordingKetoWriter) WriteRelation(_ context.Context, _, _, _, subjectID string) error {
	m.subjectIDs = append(m.subjectIDs, subjectID)
	return nil
}

func (m *recordingKetoWriter) WriteSubjectSetRelation(_ context.Context, _, _, _, _, _, _ string) error {
	m.subjectSets++
	return nil
}

func (m *recordingKetoWriter) DeleteRelation(_ context.Context, _, _, _, _ string) error {
	return nil
}
