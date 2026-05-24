package cli

import (
	"github.com/spf13/cobra"

	"github.com/callsignmedia/lono/internal/engine"
)

func (a *app) newStateCmd() *cobra.Command {
	return &cobra.Command{
		Use: "state <instance>", Short: "Show full state + available actions", Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			def, st, err := loadDefForInstance(a.store(), args[0])
			if err != nil {
				return a.emit(c, "state", nil, coded("NOT_FOUND", err, nil))
			}
			data, err := stateData(def, st, nil)
			if err != nil {
				return a.emit(c, "state", nil, coded("ERROR", err, nil))
			}
			return a.emit(c, "state", data, nil)
		},
	}
}

func (a *app) newActionsCmd() *cobra.Command {
	return &cobra.Command{
		Use: "actions <instance>", Short: "List available actions", Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			def, st, err := loadDefForInstance(a.store(), args[0])
			if err != nil {
				return a.emit(c, "actions", nil, coded("NOT_FOUND", err, nil))
			}
			actions, err := engine.AvailableActions(def, st)
			if err != nil {
				return a.emit(c, "actions", nil, coded("ERROR", err, nil))
			}
			return a.emit(c, "actions", map[string]any{"actions": actions}, nil)
		},
	}
}
