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
	ctx := cuecontext.New()

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve cue dir: %w", err)
	}

	// Load all CUE files under the auth directory as a single package.
	cfg := &load.Config{
		Dir: absDir,
	}
	instances := load.Instances([]string{"./..."}, cfg)
	if len(instances) == 0 {
		return nil, fmt.Errorf("no CUE instances found in %s", absDir)
	}

	// Unify all instances into a single value.
	var unified cue.Value
	for i, inst := range instances {
		if inst.Err != nil {
			return nil, fmt.Errorf("load cue instance %d: %w", i, inst.Err)
		}
		val := ctx.BuildInstance(inst)
		if val.Err() != nil {
			return nil, fmt.Errorf("build cue instance %d: %w", i, val.Err())
		}
		if i == 0 {
			unified = val
		} else {
			unified = unified.Unify(val)
		}
	}

	if err := unified.Validate(cue.Final()); err != nil {
		return nil, fmt.Errorf("validate cue: %w", err)
	}

	result := &CUEConfig{}

	// Extract registry commands.
	regVal := unified.LookupPath(cue.ParsePath("registry"))
	if regVal.Exists() {
		iter, err := regVal.Fields()
		if err != nil {
			return nil, fmt.Errorf("iterate registry: %w", err)
		}
		for iter.Next() {
			var cmd Command
			if err := iter.Value().Decode(&cmd); err != nil {
				return nil, fmt.Errorf("decode command %s: %w", iter.Selector().String(), err)
			}
			c := cmd
			result.Commands = append(result.Commands, &c)
		}
	}

	// Extract roles.
	rolesVal := unified.LookupPath(cue.ParsePath("roles"))
	if rolesVal.Exists() {
		iter, err := rolesVal.Fields()
		if err != nil {
			return nil, fmt.Errorf("iterate roles: %w", err)
		}
		for iter.Next() {
			var role CUERole
			if err := iter.Value().Decode(&role); err != nil {
				return nil, fmt.Errorf("decode role %s: %w", iter.Selector().String(), err)
			}
			result.Roles = append(result.Roles, role)
		}
	}

	// Extract actors (optional).
	actorsVal := unified.LookupPath(cue.ParsePath("actors"))
	if actorsVal.Exists() {
		iter, err := actorsVal.Fields()
		if err != nil {
			return nil, fmt.Errorf("iterate actors: %w", err)
		}
		for iter.Next() {
			var actor CUEActor
			if err := iter.Value().Decode(&actor); err != nil {
				return nil, fmt.Errorf("decode actor %s: %w", iter.Selector().String(), err)
			}
			result.Actors = append(result.Actors, actor)
		}
	}

	return result, nil
}
