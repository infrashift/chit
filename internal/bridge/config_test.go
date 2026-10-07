package bridge

import "testing"

// Exactly one authentication mode: the agent acts either as an OAuth2
// machine actor or as a Kratos identity, never "whichever happens to win".
func TestConfig_AuthMode(t *testing.T) {
	oauth := map[string]string{
		"CHIT_CLAUDE_OAUTH_CLIENT_ID":     "agent",
		"CHIT_CLAUDE_OAUTH_CLIENT_SECRET": "secret",
		"CHIT_CLAUDE_OAUTH_TOKEN_URL":     "http://hydra/oauth2/token",
	}
	kratos := map[string]string{"CHIT_CLAUDE_AGENT_KRATOS_ID": "019421a0-0000-7000-8000-000000000001"}
	merge := func(ms ...map[string]string) map[string]string {
		out := map[string]string{}
		for _, m := range ms {
			for k, v := range m {
				out[k] = v
			}
		}
		return out
	}

	cases := []struct {
		name string
		env  map[string]string
		ok   bool
	}{
		{"OAuth only", oauth, true},
		{"Kratos identity only", kratos, true},
		{"both", merge(oauth, kratos), false},
		{"neither", map[string]string{}, false},
		{"OAuth without its secret", merge(oauth, map[string]string{"CHIT_CLAUDE_OAUTH_CLIENT_SECRET": ""}), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Blank every auth variable first, so the developer's own
			// environment cannot decide the outcome.
			base := map[string]string{
				"CHIT_CLAUDE_OAUTH_CLIENT_ID": "", "CHIT_CLAUDE_OAUTH_CLIENT_SECRET": "",
				"CHIT_CLAUDE_OAUTH_TOKEN_URL": "", "CHIT_CLAUDE_AGENT_KRATOS_ID": "",
				"CHIT_CLAUDE_CHANNELS": "019421a0-0000-7000-8000-000000000020",
				"CHIT_CLAUDE_WORKDIR":  t.TempDir(),
			}
			for k, v := range merge(base, tc.env) {
				t.Setenv(k, v)
			}
			_, err := Load()
			if (err == nil) != tc.ok {
				t.Fatalf("err = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}
