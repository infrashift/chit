package config

import "testing"

func TestDefaults_EnableOpenAPIValidation(t *testing.T) {
	cfg := Defaults()
	if !cfg.EnableOpenAPIValidation {
		t.Fatal("expected EnableOpenAPIValidation to default to true")
	}
}

