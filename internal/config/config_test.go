package config

import "testing"

func TestDefaults_EnableOpenAPIValidation(t *testing.T) {
	cfg := Defaults()
	if !cfg.EnableOpenAPIValidation {
		t.Fatal("expected EnableOpenAPIValidation to default to true")
	}
}

func TestDefaults_MCPAgentUserID(t *testing.T) {
	cfg := Defaults()
	if cfg.MCPAgentUserID != "" {
		t.Fatalf("expected MCPAgentUserID to default to empty string, got %q", cfg.MCPAgentUserID)
	}
}
