package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/callsignmedia/lono/internal/engine"
)

func (a *app) newAdvanceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "advance <instance> [n]",
		Short: "Advance the instance clock by n ticks (default 1), firing scheduled effects and triggers",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(c *cobra.Command, args []string) error {
			instanceID := args[0]
			n := 1
			if len(args) == 2 {
				parsed, err := strconv.Atoi(args[1])
				if err != nil || parsed <= 0 {
					return a.emit(c, "advance", nil,
						coded("BAD_INPUT", fmt.Errorf("n must be a positive integer, got %q", args[1]), nil))
				}
				n = parsed
			}

			s := a.store()
			release, err := s.Lock(instanceID)
			if err != nil {
				return a.emit(c, "advance", nil, coded("LOCKED", err, nil))
			}
			defer release()

			def, st, err := loadDefForInstance(s, instanceID)
			if err != nil {
				return a.emit(c, "advance", nil, coded("NOT_FOUND", err, nil))
			}

			ns, res, err := engine.AdvanceInstance(def, st, n)
			if err != nil {
				return a.emit(c, "advance", nil, coded("APPLY_FAILED", err, nil))
			}
			if err := s.SaveState(ns); err != nil {
				return a.emit(c, "advance", nil, coded("IO_ERROR", err, nil))
			}
			data, err := stateData(def, ns, map[string]any{
				"fired":    res.Fired,
				"warnings": res.Warnings,
			})
			if err != nil {
				return a.emit(c, "advance", nil, coded("ERROR", err, nil))
			}
			return a.emit(c, "advance", data, nil)
		},
	}
}
