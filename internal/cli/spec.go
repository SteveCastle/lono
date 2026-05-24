package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// addSpecFlags adds --spec and --spec-file to a command.
func addSpecFlags(c *cobra.Command) {
	c.Flags().String("spec", "", "inline JSON spec")
	c.Flags().String("spec-file", "", "path to a JSON spec file")
}

// readSpec returns the JSON bytes from --spec or --spec-file (exactly one).
func readSpec(c *cobra.Command) ([]byte, error) {
	inline, _ := c.Flags().GetString("spec")
	file, _ := c.Flags().GetString("spec-file")
	switch {
	case inline != "" && file != "":
		return nil, fmt.Errorf("use only one of --spec or --spec-file")
	case inline != "":
		return []byte(inline), nil
	case file != "":
		return os.ReadFile(file)
	default:
		return nil, fmt.Errorf("provide --spec or --spec-file")
	}
}
