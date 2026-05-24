package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/callsignmedia/lono/internal/engine"
)

func (a *app) newInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect <instance> [path]",
		Short: "Inspect a running game instance by path",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(c *cobra.Command, args []string) error {
			instance := args[0]
			useTree, _ := c.Flags().GetBool("tree")
			depth, _ := c.Flags().GetInt("depth")

			if useTree {
				// --tree branch: raw state only (no def needed).
				st, err := a.store().LoadState(instance)
				if err != nil {
					return a.emit(c, "inspect", nil, coded("NOT_FOUND", err, nil))
				}
				b, _ := json.Marshal(st)
				var m any
				_ = json.Unmarshal(b, &m)
				return a.emit(c, "inspect", map[string]any{"tree": engine.TreeView(m, depth)}, nil)
			}

			def, st, err := loadDefForInstance(a.store(), instance)
			if err != nil {
				return a.emit(c, "inspect", nil, coded("NOT_FOUND", err, nil))
			}
			path := ""
			if len(args) == 2 {
				path = args[1]
			}
			v, err := engine.GetStateNode(def, st, path)
			if err != nil {
				if engine.IsNoSuchPath(err) {
					return a.emit(c, "inspect", nil, coded("NO_SUCH_PATH", err, nil))
				}
				return a.emit(c, "inspect", nil, coded("ERROR", err, nil))
			}
			return a.emit(c, "inspect", map[string]any{"path": path, "value": v}, nil)
		},
	}
	cmd.Flags().Bool("tree", false, "return names-only structural tree instead of a value")
	cmd.Flags().Int("depth", 2, "depth limit for --tree (default 2)")
	return cmd
}
