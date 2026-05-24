package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

// runCLI executes the root command with args (always JSON output to a temp dir)
// and returns the parsed envelope plus the raw stdout.
func runCLI(t *testing.T, dataDir string, args ...string) (Envelope, string) {
	t.Helper()
	full := append([]string{"--data-dir", dataDir}, args...)
	cmd := NewRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(full)
	_ = cmd.Execute()
	var env Envelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("output not a JSON envelope: %v\n%s", err, out.String())
	}
	return env, out.String()
}

func TestVersionEnvelope(t *testing.T) {
	env, _ := runCLI(t, t.TempDir(), "version")
	if !env.OK || env.Command != "version" {
		t.Fatalf("bad envelope: %+v", env)
	}
}
