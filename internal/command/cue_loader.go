package command

import (
	"fmt"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// CUERole represents a role loaded from CUE.
type CUERole struct {
	Name            string   `json:"name"`
	AllowedCommands []string `json:"allowed_commands"`
}

// CUEActor represents an actor binding loaded from CUE.
type CUEActor struct {
	ID    string   `json:"id"`
	Roles []string `json:"roles"`
}

// CUEConfig holds the fully resolved configuration from CUE files.
type CUEConfig struct {
	Commands []*Command
	Roles    []CUERole
	Actors   []CUEActor
}

// LoadCUE reads and validates CUE files from the given directory tree.
// It expects the directory to contain schema/ and roles/ subdirectories
// with a shared "auth" package.
func LoadCUE(dir string) (*CUEConfig, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve cue dir: %w", err)
	}

	unified, err := loadUnifiedCUE(absDir)
	if err != nil {
		return nil, err
	}

	if err = unified.Validate(cue.Final()); err != nil {
		return nil, fmt.Errorf("validate cue: %w", err)
	}

	result := &CUEConfig{}

	// Extract registry commands.
	commands, err := decodeCUEFields[Command](unified, "registry", "command")
	if err != nil {
		return nil, err
	}
	for i := range commands {
		result.Commands = append(result.Commands, &commands[i])
	}

	// Extract roles.
	if result.Roles, err = decodeCUEFields[CUERole](unified, "roles", "role"); err != nil {
		return nil, err
	}

	// Extract actors (optional).
	if result.Actors, err = decodeCUEFields[CUEActor](unified, "actors", "actor"); err != nil {
		return nil, err
	}

	return result, nil
}

// loadUnifiedCUE loads all CUE files under absDir as a single package and
// unifies the instances into a single value.
func loadUnifiedCUE(absDir string) (cue.Value, error) {
	ctx := cuecontext.New()

	// Load all CUE files under the auth directory as a single package.
	cfg := &load.Config{
		Dir: absDir,
	}
	instances := load.Instances([]string{"./..."}, cfg)
	if len(instances) == 0 {
		return cue.Value{}, fmt.Errorf("no CUE instances found in %s", absDir)
	}

	// Unify all instances into a single value.
	var unified cue.Value
	for i, inst := range instances {
		if inst.Err != nil {
			return cue.Value{}, fmt.Errorf("load cue instance %d: %w", i, inst.Err)
		}
		val := ctx.BuildInstance(inst)
		if val.Err() != nil {
			return cue.Value{}, fmt.Errorf("build cue instance %d: %w", i, val.Err())
		}
		if i == 0 {
			unified = val
		} else {
			unified = unified.Unify(val)
		}
	}
	return unified, nil
}

// decodeCUEFields decodes each field of the struct at path into a T, in
// field order. A path that does not exist yields no values. what names one
// value in error messages; the path names the struct ("iterate <path>").
func decodeCUEFields[T any](unified cue.Value, path, what string) ([]T, error) {
	val := unified.LookupPath(cue.ParsePath(path))
	if !val.Exists() {
		return nil, nil
	}
	iter, err := val.Fields()
	if err != nil {
		return nil, fmt.Errorf("iterate %s: %w", path, err)
	}
	var out []T
	for iter.Next() {
		var item T
		if err := iter.Value().Decode(&item); err != nil {
			return nil, fmt.Errorf("decode %s %s: %w", what, iter.Selector().String(), err)
		}
		out = append(out, item)
	}
	return out, nil
}
