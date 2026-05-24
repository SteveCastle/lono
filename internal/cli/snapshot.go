package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/callsignmedia/lono/internal/store"
)

func (a *app) newSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "snapshot", Short: "Capture and restore instance state"}

	create := &cobra.Command{
		Use: "create <instance>", Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			s := a.store()
			release, err := s.Lock(args[0])
			if err != nil {
				return a.emit(c, "snapshot.create", nil, coded("LOCKED", err, nil))
			}
			defer release()
			st, err := s.LoadState(args[0])
			if err != nil {
				return a.emit(c, "snapshot.create", nil, coded("NOT_FOUND", err, nil))
			}
			id, _ := c.Flags().GetString("id")
			if id == "" {
				id = fmt.Sprintf("snap-%d", time.Now().UnixNano())
			}
			label, _ := c.Flags().GetString("label")
			snap := &store.Snapshot{ID: id, Label: label, State: st, CreatedAt: time.Now().UTC()}
			if err := s.SaveSnapshot(args[0], snap); err != nil {
				return a.emit(c, "snapshot.create", nil, coded("IO_ERROR", err, nil))
			}
			return a.emit(c, "snapshot.create", map[string]any{"id": id, "label": label}, nil)
		},
	}
	create.Flags().String("id", "", "snapshot id (default generated)")
	create.Flags().String("label", "", "human-readable label")

	list := &cobra.Command{
		Use: "list <instance>", Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			metas, err := a.store().ListSnapshots(args[0])
			if err != nil {
				return a.emit(c, "snapshot.list", nil, coded("IO_ERROR", err, nil))
			}
			return a.emit(c, "snapshot.list", map[string]any{"snapshots": metas}, nil)
		},
	}

	show := &cobra.Command{
		Use: "show <instance> <snap>", Args: cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			snap, err := a.store().LoadSnapshot(args[0], args[1])
			if err != nil {
				return a.emit(c, "snapshot.show", nil, coded("NOT_FOUND", err, nil))
			}
			return a.emit(c, "snapshot.show", snap, nil)
		},
	}

	restore := &cobra.Command{
		Use: "restore <instance> <snap>", Args: cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			inPlace, _ := c.Flags().GetBool("in-place")
			into, _ := c.Flags().GetString("into")
			if inPlace && into != "" {
				return a.emit(c, "snapshot.restore", nil, coded("BAD_INPUT",
					fmt.Errorf("use only one of --in-place or --into"), nil))
			}
			s := a.store()
			snap, err := s.LoadSnapshot(args[0], args[1])
			if err != nil {
				return a.emit(c, "snapshot.restore", nil, coded("NOT_FOUND", err, nil))
			}
			target := args[0]
			if !inPlace {
				if into == "" {
					into = fmt.Sprintf("%s-restore-%d", args[0], time.Now().UnixNano())
				}
				target = into
				if _, err := s.LoadState(target); err == nil {
					return a.emit(c, "snapshot.restore", nil, coded("INSTANCE_EXISTS",
						fmt.Errorf("branch target %q already exists", target), nil))
				}
			}
			// Lock the instance we're about to write, mirroring do/apply.
			release, err := s.Lock(target)
			if err != nil {
				return a.emit(c, "snapshot.restore", nil, coded("LOCKED", err, nil))
			}
			defer release()
			restored := snap.State.Clone()
			restored.ID = target
			restored.UpdatedAt = time.Now().UTC()
			if err := s.SaveState(restored); err != nil {
				return a.emit(c, "snapshot.restore", nil, coded("IO_ERROR", err, nil))
			}
			return a.emit(c, "snapshot.restore", map[string]any{"instance": target, "inPlace": inPlace}, nil)
		},
	}
	restore.Flags().Bool("in-place", false, "overwrite the current instance instead of branching")
	restore.Flags().String("into", "", "new instance id to branch into (default generated)")

	cmd.AddCommand(create, list, show, restore)
	return cmd
}
