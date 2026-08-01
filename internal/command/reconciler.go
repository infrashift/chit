package command

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/infrashift/chit/internal/model"
)

// KetoWriter abstracts Keto write operations for testing.
type KetoWriter interface {
	WriteRelation(ctx context.Context, namespace, object, relation, subjectID string) error
	// WriteSubjectSetRelation writes a tuple whose subject is another
	// relation rather than a literal ID. Keto only expands a subject when it
	// is stored this way; a subject_id of "Role:admin#member" is an opaque
	// string that matches nobody.
	WriteSubjectSetRelation(ctx context.Context, namespace, object, relation,
		subjectNamespace, subjectObject, subjectRelation string) error
	DeleteRelation(ctx context.Context, namespace, object, relation, subjectID string) error
}

// Reconciler syncs CUE-defined AuthZ tuples to Keto.
type Reconciler struct {
	keto KetoWriter
}

// NewReconciler creates a Reconciler backed by the given KetoWriter.
func NewReconciler(keto KetoWriter) *Reconciler {
	return &Reconciler{keto: keto}
}

// Reconcile pushes role→command and actor→role tuples to Keto.
func (r *Reconciler) Reconcile(ctx context.Context, cfg *CUEConfig) error {
	ns := model.KetoNamespaceCommand
	rel := model.KetoRelationExecute

	// 1. For each role, grant execute on each allowed command via role
	// membership. The subject is the role's member set, not a literal ID, so
	// that Keto expands it to the actors bound to that role in step 2.
	for _, role := range cfg.Roles {
		roleObject := fmt.Sprintf("Role:%s", role.Name)
		for _, cmdSlug := range role.AllowedCommands {
			object := fmt.Sprintf("Command:%s", cmdSlug)
			slog.Info("reconcile: granting command to role",
				"command", cmdSlug, "role", role.Name)
			if err := r.keto.WriteSubjectSetRelation(ctx, ns, object, rel,
				ns, roleObject, model.KetoRelationMember); err != nil {
				return fmt.Errorf("write command grant (cmd=%s, role=%s): %w", cmdSlug, role.Name, err)
			}
		}
	}

	// 2. For each actor, assign them as a member of their roles.
	for _, actor := range cfg.Actors {
		actorSubject := fmt.Sprintf("Actor:%s", actor.ID)
		for _, roleName := range actor.Roles {
			object := fmt.Sprintf("Role:%s", roleName)
			slog.Info("reconcile: assigning actor to role",
				"actor", actor.ID, "role", roleName)
			if err := r.keto.WriteRelation(ctx, ns, object, model.KetoRelationMember, actorSubject); err != nil {
				return fmt.Errorf("write role membership (actor=%s, role=%s): %w", actor.ID, roleName, err)
			}
		}
	}

	return nil
}
