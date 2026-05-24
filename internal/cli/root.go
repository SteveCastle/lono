package cli

import "github.com/spf13/cobra"

type globalFlags struct {
	dataDir string
	format  string
	pretty  bool
}

func NewRootCmd() *cobra.Command {
	a := &app{gf: &globalFlags{}}
	root := &cobra.Command{
		Use:           "lono",
		Short:         "Track state for story-driven games",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&a.gf.dataDir, "data-dir", "", "data directory (default $LONO_HOME or ./.lono)")
	root.PersistentFlags().StringVar(&a.gf.format, "format", "json", "output format: json|text")
	root.PersistentFlags().BoolVar(&a.gf.pretty, "pretty", false, "pretty-print JSON output")

	version := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.emit(cmd, "version", map[string]string{"version": "0.1.0"}, nil)
		},
	}
	root.AddCommand(version)
	root.AddCommand(a.newGameCmd())
	root.AddCommand(a.newDefineCmd())
	root.AddCommand(a.newPlayCmd())
	root.AddCommand(a.newStateCmd())
	root.AddCommand(a.newActionsCmd())
	root.AddCommand(a.newDoCmd())
	root.AddCommand(a.newApplyCmd())
	root.AddCommand(a.newSnapshotCmd())
	root.AddCommand(a.newInspectCmd())
	root.AddCommand(a.newSetCmd())
	root.AddCommand(a.newRmCmd())
	return root
}
