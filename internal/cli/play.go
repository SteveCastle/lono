package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/callsignmedia/lono/internal/engine"
)

func (a *app) newPlayCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "play", Short: "Start and list game instances"}

	start := &cobra.Command{
		Use: "start <game>", Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			def, err := a.store().LoadDefinition(args[0])
			if err != nil {
				return a.emit(c, "play.start", nil, coded("NOT_FOUND", err, nil))
			}
			id, _ := c.Flags().GetString("id")
			if id == "" {
				id = newInstanceID(def.ID)
			}
			if force, _ := c.Flags().GetBool("force"); !force {
				if _, err := a.store().LoadState(id); err == nil {
					return a.emit(c, "play.start", nil, coded("INSTANCE_EXISTS",
						fmt.Errorf("instance %q already exists (use --force to overwrite)", id), nil))
				}
			}
			seed, _ := c.Flags().GetInt64("seed")
			if seed == 0 {
				seed = time.Now().UnixNano()
			}
			st, err := engine.StartInstance(def, id, seed)
			if err != nil {
				return a.emit(c, "play.start", nil, coded("SETUP_FAILED", err, nil))
			}
			if err := a.store().SaveState(st); err != nil {
				return a.emit(c, "play.start", nil, coded("IO_ERROR", err, nil))
			}
			data, err := stateData(def, st, nil)
			if err != nil {
				return a.emit(c, "play.start", nil, coded("ERROR", err, nil))
			}
			return a.emit(c, "play.start", data, nil)
		},
	}
	start.Flags().String("id", "", "instance id (default generated)")
	start.Flags().Int64("seed", 0, "RNG seed (default time-based)")
	start.Flags().Bool("force", false, "overwrite an existing instance with the same id")

	list := &cobra.Command{
		Use: "list", Short: "List instance ids",
		RunE: func(c *cobra.Command, args []string) error {
			ids, err := a.store().ListInstances()
			if err != nil {
				return a.emit(c, "play.list", nil, coded("IO_ERROR", err, nil))
			}
			return a.emit(c, "play.list", map[string]any{"instances": ids}, nil)
		},
	}

	cmd.AddCommand(start, list)
	return cmd
}
