package keto_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/parser"
	"gopkg.in/yaml.v3"

	"github.com/infrashift/chit/internal/model"
)

const (
	schemaFile = "../../deploy/keto/schema/chit.cue"
	configFile = "../../deploy/keto/keto.yml"
)

var kubeFiles = []string{"../../deploy/chit.kube.yml", "../../deploy/chit-uat.kube.yml"}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// configMap returns the data of the named ConfigMap in a kube file.
func configMap(t *testing.T, path, name string) map[string]string {
	t.Helper()
	dec := yaml.NewDecoder(bytes.NewReader([]byte(readFile(t, path))))
	for {
		var doc struct {
			Kind     string `yaml:"kind"`
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
			Data map[string]string `yaml:"data"`
		}
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			t.Fatalf("%s: no %s ConfigMap", path, name)
		}
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if doc.Kind == "ConfigMap" && doc.Metadata.Name == name {
			return doc.Data
		}
	}
}

// TestKetoSchemaInSync keeps the ConfigMap copies of the keto-cue config and
// schema the same as deploy/keto, which `make keto-validate` and
// `make test-keto` use.
func TestKetoSchemaInSync(t *testing.T) {
	want := []struct{ configMap, key, file string }{
		{"chit-keto-config", "keto.yml", configFile},
		{"chit-keto-schema", "chit.cue", schemaFile},
	}
	for _, kube := range kubeFiles {
		for _, w := range want {
			data := configMap(t, kube, w.configMap)
			if len(data) != 1 || data[w.key] != readFile(t, w.file) {
				t.Errorf("%s: %s must hold only %q, a copy of %s", kube, w.configMap, w.key, w.file)
			}
		}
	}
}

// TestKetoSchemaSharesThePackage checks the rules that let Chit's file share
// a keto-cue schema directory with other services' files: keto-cue compiles
// them as one package, so the package must be the shared name, and every
// namespace must carry Chit's prefix, or CUE would merge it with another
// service's namespace of the same name. `make keto-validate` compiles the
// file next to a sample HALO file.
func TestKetoSchemaSharesThePackage(t *testing.T) {
	f, err := parser.ParseFile(schemaFile, readFile(t, schemaFile))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.PackageName(); got != "keto" {
		t.Errorf("package %q, want keto: keto-cue refuses two package names in one directory", got)
	}
	v := cuecontext.New().BuildFile(f)
	iter, err := v.LookupPath(cue.ParsePath("namespaces")).Fields()
	if err != nil {
		t.Fatal(err)
	}
	for iter.Next() {
		if name := iter.Selector().Unquoted(); !strings.HasPrefix(name, "chit/") {
			t.Errorf("namespace %q lacks the chit/ prefix", name)
		}
	}
}

// TestKetoSchemaDeclaresWhatChitUses compiles the schema and checks that it
// declares every namespace and relation Chit writes or checks. keto-cue
// refuses a write to an undeclared namespace (404) and a check of an
// undeclared relation (400), so a constant renamed in internal/model without
// the schema would break authorization at runtime.
func TestKetoSchemaDeclaresWhatChitUses(t *testing.T) {
	v := cuecontext.New().CompileString(readFile(t, schemaFile), cue.Filename(schemaFile))
	if err := v.Err(); err != nil {
		t.Fatalf("compile %s: %v", schemaFile, err)
	}

	types := func(ns, rel string) []string {
		t.Helper()
		var out []string
		path := cue.ParsePath("namespaces").Append(cue.Str(ns), cue.Str("relations"), cue.Str(rel))
		if err := v.LookupPath(path).Decode(&out); err != nil {
			t.Fatalf("relation %s#%s: %v", ns, rel, err)
		}
		return out
	}

	types(model.KetoNamespaceChannel, model.KetoRelationMember)
	types(model.KetoNamespaceCommand, model.KetoRelationMember)
	// The reconciler grants a command to a role's member set; keto-cue's
	// strict mode expands only subject sets that the relation declares.
	execute := types(model.KetoNamespaceCommand, model.KetoRelationExecute)
	if want := model.KetoNamespaceCommand + "#" + model.KetoRelationMember; !slices.Contains(execute, want) {
		t.Errorf("%s#%s types = %v, want %q", model.KetoNamespaceCommand, model.KetoRelationExecute, execute, want)
	}
}
