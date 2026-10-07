package command

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/infrashift/chit/internal/keto"
	"github.com/infrashift/chit/internal/model"
)

// KetoClient is the slice of Keto the reconciler uses.
type KetoClient interface {
	WriteRelation(ctx context.Context, namespace, object, relation, subjectID string) error
	// WriteSubjectSetRelation writes a tuple whose subject is another
	// relation rather than a literal ID. Keto only expands a subject when it
	// is stored this way; a subject_id of "Role:admin#member" is an opaque
	// string that matches nobody.
	WriteSubjectSetRelation(ctx context.Context, namespace, object, relation,
		subjectNamespace, subjectObject, subjectRelation string) error
	ListRelations(ctx context.Context, namespace string) ([]keto.Tuple, error)
	DeleteTuple(ctx context.Context, t *keto.Tuple) error
}

// Reconciler makes Keto's command namespace match the CUE definitions.
type Reconciler struct {
	keto KetoClient
}

// NewReconciler creates a Reconciler backed by the given Keto client.
func NewReconciler(k KetoClient) *Reconciler {
	return &Reconciler{keto: k}
}

// Reconcile makes the command namespace hold exactly the tuples the CUE
// declares: role→command grants and actor→role memberships.
//
// It writes every declared tuple, then deletes every other tuple in the
// namespace. It used to only write, so taking a command away from a role, or
// a role away from an actor, in the CUE changed nothing in Keto: the grant
// stayed in force until someone deleted it by hand. The namespace belongs to
// the reconciler (nothing else writes chit/command), so anything there that
// the CUE does not declare is a revoked grant.
//
// Writes come first so that a grant being kept is never briefly missing. If
// listing fails, nothing is deleted.
func (r *Reconciler) Reconcile(ctx context.Context, cfg *CUEConfig) error {
	ns := model.KetoNamespaceCommand
	desired := map[string]bool{}

	// 1. For each role, grant execute on each allowed command via role
	// membership. The subject is the role's member set, not a literal ID, so
	// that Keto expands it to the actors bound to that role in step 2.
	for _, role := range cfg.Roles {
		roleObject := fmt.Sprintf("Role:%s", role.Name)
		for _, cmdSlug := range role.AllowedCommands {
			t := keto.Tuple{
				Namespace: ns, Object: fmt.Sprintf("Command:%s", cmdSlug), Relation: model.KetoRelationExecute,
				SubjectSet: &keto.SubjectSet{Namespace: ns, Object: roleObject, Relation: model.KetoRelationMember},
			}
			desired[t.String()] = true
			slog.Info("reconcile: granting command to role", "command", cmdSlug, "role", role.Name)
			if err := r.keto.WriteSubjectSetRelation(ctx, t.Namespace, t.Object, t.Relation,
				t.SubjectSet.Namespace, t.SubjectSet.Object, t.SubjectSet.Relation); err != nil {
				return fmt.Errorf("write command grant (cmd=%s, role=%s): %w", cmdSlug, role.Name, err)
			}
		}
	}

	// 2. For each actor, assign them as a member of their roles.
	for _, actor := range cfg.Actors {
		for _, roleName := range actor.Roles {
			t := keto.Tuple{
				Namespace: ns, Object: fmt.Sprintf("Role:%s", roleName), Relation: model.KetoRelationMember,
				SubjectID: fmt.Sprintf("Actor:%s", actor.ID),
			}
			desired[t.String()] = true
			slog.Info("reconcile: assigning actor to role", "actor", actor.ID, "role", roleName)
			if err := r.keto.WriteRelation(ctx, t.Namespace, t.Object, t.Relation, t.SubjectID); err != nil {
				return fmt.Errorf("write role membership (actor=%s, role=%s): %w", actor.ID, roleName, err)
			}
		}
	}

	// 3. Revoke everything else in the namespace.
	existing, err := r.keto.ListRelations(ctx, ns)
	if err != nil {
		return fmt.Errorf("list %s tuples: %w", ns, err)
	}
	for i := range existing {
		t := &existing[i]
		if desired[t.String()] {
			continue
		}
		slog.Info("reconcile: revoking tuple no longer declared", "tuple", t.String())
		if err := r.keto.DeleteTuple(ctx, t); err != nil {
			return fmt.Errorf("revoke %s: %w", t.String(), err)
		}
	}

	return nil
}
