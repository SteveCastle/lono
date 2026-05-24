package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/callsignmedia/lono/internal/engine"
)

func (a *app) newDoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "do <instance> <machine> <action>", Short: "Perform an action", Args: cobra.ExactArgs(3),
		RunE: func(c *cobra.Command, args []string) error {
			instance, machine, action := args[0], args[1], args[2]
			params := map[string]any{}
			if raw, _ := c.Flags().GetString("params"); raw != "" {
				if err := json.Unmarshal([]byte(raw), &params); err != nil {
					return a.emit(c, "do", nil, coded("BAD_INPUT", err, nil))
				}
			}
			entity, _ := c.Flags().GetString("entity")
			rel, _ := c.Flags().GetStringSlice("rel")
			var host *engine.HostRef
			switch {
			case entity != "" && len(rel) > 0:
				return a.emit(c, "do", nil, coded("BAD_INPUT", fmt.Errorf("use only one of --entity or --rel"), nil))
			case entity != "":
				host = &engine.HostRef{Kind: "entity", ID: entity}
			case len(rel) == 2:
				host = &engine.HostRef{Kind: "relationship", From: rel[0], To: rel[1]}
			case len(rel) != 0:
				return a.emit(c, "do", nil, coded("BAD_INPUT", fmt.Errorf("--rel requires exactly two values: <from> <to>"), nil))
			}

			s := a.store()
			release, err := s.Lock(instance)
			if err != nil {
				return a.emit(c, "do", nil, coded("LOCKED", err, nil))
			}
			defer release()

			def, st, err := loadDefForInstance(s, instance)
			if err != nil {
				return a.emit(c, "do", nil, coded("NOT_FOUND", err, nil))
			}
			var ns *engine.State
			var res *engine.ActionResult
			if host != nil {
				ns, res, err = engine.PerformHostAction(def, st, machine, action, params, host)
			} else {
				ns, res, err = engine.PerformAction(def, st, machine, action, params)
			}
			if err != nil {
				return a.emit(c, "do", nil, coded("ACTION_FAILED", err, nil))
			}
			if err := s.SaveState(ns); err != nil {
				return a.emit(c, "do", nil, coded("IO_ERROR", err, nil))
			}
			data, err := stateData(def, ns, map[string]any{
				"rolls":    res.Rolls,
				"fired":    res.Fired,
				"warnings": res.Warnings,
			})
			if err != nil {
				return a.emit(c, "do", nil, coded("ERROR", err, nil))
			}
			return a.emit(c, "do", data, nil)
		},
	}
	cmd.Flags().String("params", "", "action parameters as JSON object")
	cmd.Flags().String("entity", "", "host entity id (for an entity-attached machine)")
	cmd.Flags().StringSlice("rel", nil, "host relationship endpoints: --rel <from> <to> (for a relationship-attached machine)")
	return cmd
}
