package cli

import (
	"encoding/json"
	"errors"

	"github.com/spf13/cobra"

	"github.com/callsignmedia/lono/internal/store"
)

type Envelope struct {
	OK       bool     `json:"ok"`
	Command  string   `json:"command"`
	Data     any      `json:"data,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Error    *ErrInfo `json:"error,omitempty"`
}

type ErrInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// cmdError carries a structured error code and optional details.
type cmdError struct {
	code    string
	err     error
	details any
}

func (e *cmdError) Error() string { return e.err.Error() }
func (e *cmdError) Unwrap() error { return e.err }

func coded(code string, err error, details any) *cmdError {
	return &cmdError{code: code, err: err, details: details}
}

// app holds shared CLI state.
type app struct{ gf *globalFlags }

func (a *app) store() *store.Store { return store.Open(a.gf.dataDir) }

// emit writes a JSON (or text) envelope and returns runErr so the process exits
// non-zero on failure while still printing structured output.
func (a *app) emit(cmd *cobra.Command, name string, data any, runErr error) error {
	env := Envelope{Command: name}
	if runErr != nil {
		code := "ERROR"
		var details any
		var ce *cmdError
		if errors.As(runErr, &ce) {
			code = ce.code
			details = ce.details
		}
		env.Error = &ErrInfo{Code: code, Message: runErr.Error(), Details: details}
	} else {
		env.OK = true
		env.Data = data
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	if a.gf.pretty || a.gf.format == "text" {
		enc.SetIndent("", "  ")
	}
	_ = enc.Encode(env)
	return runErr
}
