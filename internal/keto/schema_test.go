package keto_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"slices"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"gopkg.in/yaml.v3"

	"github.com/infrashift/chit/internal/model"
)

const (
	schemaFile = "../../deploy/keto/namespaces.cue"
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

// ketoConfigMap returns the data of the chit-keto-config ConfigMap in a kube
// file.
func ketoConfigMap(t *testing.T, path string) map[string]string {
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
			t.Fatalf("%s: no chit-keto-config ConfigMap", path)
		}
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if doc.Kind == "ConfigMap" && doc.Metadata.Name == "chit-keto-config" {
			return doc.Data
		}
	}
}

// TestKetoSchemaInSync keeps the ConfigMap copies of the keto-cue schema and
// config the same as deploy/keto, which `make keto-validate` and
// `make test-keto` use.
func TestKetoSchemaInSync(t *testing.T) {
	want := map[string]string{
		"namespaces.cue": readFile(t, schemaFile),
		"keto.yml":       readFile(t, configFile),
	}
	for _, kube := range kubeFiles {
		data := ketoConfigMap(t, kube)
		for key, content := range want {
			if data[key] != content {
				t.Errorf("%s: chit-keto-config %q differs from deploy/keto/%s", kube, key, key)
			}
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
