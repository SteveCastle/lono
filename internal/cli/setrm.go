package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/callsignmedia/lono/internal/engine"
)

// parseValue parses a --value string: try JSON first (so numbers, bools,
// objects decode properly), fall back to treating it as a raw string.
func parseValue(raw string) any {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err == nil {
		return v
	}
	return raw
}

func (a *app) newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <instance> <path>",
		Short: "Set a runtime state path to a value (validated by default; --force bypasses validation)",
		Args:  cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			instanceID, path := args[0], args[1]
			force, _ := c.Flags().GetBool("force")

			// Resolve value from --value or --spec.
			valueChanged := c.Flags().Changed("value")
			specChanged := c.Flags().Changed("spec")
			var value any
			switch {
			case valueChanged && specChanged:
				return a.emit(c, "set", nil, coded("BAD_INPUT", fmt.Errorf("use only one of --value or --spec"), nil))
			case valueChanged:
				valueStr, _ := c.Flags().GetString("value")
				value = parseValue(valueStr)
			case specChanged:
				specStr, _ := c.Flags().GetString("spec")
				var obj map[string]any
				if err := json.Unmarshal([]byte(specStr), &obj); err != nil {
					return a.emit(c, "set", nil, coded("BAD_INPUT", fmt.Errorf("--spec must be a valid JSON object: %w", err), nil))
				}
				value = obj
			default:
				// Neither flag provided; value stays nil (valid for some paths when force is used).
			}

			s := a.store()
			release, err := s.Lock(instanceID)
			if err != nil {
				return a.emit(c, "set", nil, coded("LOCKED", err, nil))
			}
			defer release()

			def, st, err := loadDefForInstance(s, instanceID)
			if err != nil {
				return a.emit(c, "set", nil, coded("NOT_FOUND", err, nil))
			}

			if force {
				if err := engine.ForceWrite(st, path, value, false); err != nil {
					return a.emit(c, "set", nil, coded("APPLY_FAILED", err, nil))
				}
				if err := s.SaveState(st); err != nil {
					return a.emit(c, "set", nil, coded("IO_ERROR", err, nil))
				}
				data, err := stateData(def, st, nil)
				if err != nil {
					return a.emit(c, "set", nil, coded("ERROR", err, nil))
				}
				return a.emit(c, "set", data, nil)
			}

			ops, err := engine.CompileRuntimeWrite(def, st, path, value, false)
			if err != nil {
				if engine.IsPathNotWritable(err) {
					return a.emit(c, "set", nil, coded("PATH_NOT_WRITABLE", err, nil))
				}
				return a.emit(c, "set", nil, coded("BAD_INPUT", err, nil))
			}
			if len(ops) == 0 {
				// No-op (e.g. inventory already at target count) — return current state.
				data, err := stateData(def, st, nil)
				if err != nil {
					return a.emit(c, "set", nil, coded("ERROR", err, nil))
				}
				return a.emit(c, "set", data, nil)
			}
			ns, _, err := engine.ApplyOps(def, st, ops)
			if err != nil {
				return a.emit(c, "set", nil, coded("ACTION_FAILED", err, nil))
			}
			if err := s.SaveState(ns); err != nil {
				return a.emit(c, "set", nil, coded("IO_ERROR", err, nil))
			}
			data, err := stateData(def, ns, nil)
			if err != nil {
				return a.emit(c, "set", nil, coded("ERROR", err, nil))
			}
			return a.emit(c, "set", data, nil)
		},
	}
	cmd.Flags().String("value", "", "value to set (parsed as JSON; falls back to string)")
	cmd.Flags().String("spec", "", "JSON object value to set")
	cmd.Flags().Bool("force", false, "bypass validation and write raw value")
	return cmd
}

func (a *app) newRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <instance> <path>",
		Short: "Remove a runtime state path (validated by default; --force bypasses validation)",
		Args:  cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			instanceID, path := args[0], args[1]
			force, _ := c.Flags().GetBool("force")

			s := a.store()
			release, err := s.Lock(instanceID)
			if err != nil {
				return a.emit(c, "rm", nil, coded("LOCKED", err, nil))
			}
			defer release()

			def, st, err := loadDefForInstance(s, instanceID)
			if err != nil {
				return a.emit(c, "rm", nil, coded("NOT_FOUND", err, nil))
			}

			if force {
				if err := engine.ForceWrite(st, path, nil, true); err != nil {
					return a.emit(c, "rm", nil, coded("APPLY_FAILED", err, nil))
				}
				if err := s.SaveState(st); err != nil {
					return a.emit(c, "rm", nil, coded("IO_ERROR", err, nil))
				}
				data, err := stateData(def, st, nil)
				if err != nil {
					return a.emit(c, "rm", nil, coded("ERROR", err, nil))
				}
				return a.emit(c, "rm", data, nil)
			}

			ops, err := engine.CompileRuntimeWrite(def, st, path, nil, true)
			if err != nil {
				if engine.IsPathNotWritable(err) {
					return a.emit(c, "rm", nil, coded("PATH_NOT_WRITABLE", err, nil))
				}
				return a.emit(c, "rm", nil, coded("BAD_INPUT", err, nil))
			}
			if len(ops) == 0 {
				// No-op (e.g. inventory already zero).
				data, err := stateData(def, st, nil)
				if err != nil {
					return a.emit(c, "rm", nil, coded("ERROR", err, nil))
				}
				return a.emit(c, "rm", data, nil)
			}
			ns, _, err := engine.ApplyOps(def, st, ops)
			if err != nil {
				return a.emit(c, "rm", nil, coded("ACTION_FAILED", err, nil))
			}
			if err := s.SaveState(ns); err != nil {
				return a.emit(c, "rm", nil, coded("IO_ERROR", err, nil))
			}
			data, err := stateData(def, ns, nil)
			if err != nil {
				return a.emit(c, "rm", nil, coded("ERROR", err, nil))
			}
			return a.emit(c, "rm", data, nil)
		},
	}
	cmd.Flags().Bool("force", false, "bypass validation and delete raw key")
	return cmd
}
