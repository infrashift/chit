package command

import (
	"context"
	"fmt"
	"testing"
)

type mockKetoWriter struct {
	tuples []string
}

func (m *mockKetoWriter) WriteRelation(_ context.Context, namespace, object, relation, subjectID string) error {
	m.tuples = append(m.tuples, fmt.Sprintf("%s:%s#%s@%s", namespace, object, relation, subjectID))
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
	expected := map[string]bool{
		"chit/command:Command:help#execute@Role:admin#member": true,
		"chit/command:Command:kick#execute@Role:admin#member": true,
		"chit/command:Command:help#execute@Role:user#member":  true,
		"chit/command:Role:admin#member@Actor:alice-uuid":     true,
		"chit/command:Role:user#member@Actor:bob-uuid":        true,
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
