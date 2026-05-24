package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/callsignmedia/lono/internal/engine"
)

func (a *app) newApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "apply <instance>", Short: "Apply ad-hoc state updates", Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			raw, _ := c.Flags().GetString("ops")
			if raw == "" {
				return a.emit(c, "apply", nil, coded("BAD_INPUT", fmt.Errorf("--ops is required"), nil))
			}
			var ops []engine.Effect
			if err := json.Unmarshal([]byte(raw), &ops); err != nil {
				return a.emit(c, "apply", nil, coded("BAD_INPUT", err, nil))
			}
			s := a.store()
			release, err := s.Lock(args[0])
			if err != nil {
				return a.emit(c, "apply", nil, coded("LOCKED", err, nil))
			}
			defer release()

			def, st, err := loadDefForInstance(s, args[0])
			if err != nil {
				return a.emit(c, "apply", nil, coded("NOT_FOUND", err, nil))
			}
			ns, res, err := engine.ApplyOps(def, st, ops)
			if err != nil {
				return a.emit(c, "apply", nil, coded("APPLY_FAILED", err, nil))
			}
			if err := s.SaveState(ns); err != nil {
				return a.emit(c, "apply", nil, coded("IO_ERROR", err, nil))
			}
			data, err := stateData(def, ns, map[string]any{
				"rolls":    res.Rolls,
				"fired":    res.Fired,
				"warnings": res.Warnings,
			})
			if err != nil {
				return a.emit(c, "apply", nil, coded("ERROR", err, nil))
			}
			return a.emit(c, "apply", data, nil)
		},
	}
	cmd.Flags().String("ops", "", "JSON array of effect ops")
	return cmd
}
