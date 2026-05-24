package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func (a *app) newLoreCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "lore", Short: "Query the game lore / codex"}

	// lore list <game> [--tag <t>] [--subject <id>]
	list := &cobra.Command{
		Use:   "list <game>",
		Short: "List lore entries (optionally filtered by tag or subject)",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			def, err := a.store().LoadDefinition(args[0])
			if err != nil {
				return a.emit(c, "lore.list", nil, coded("NOT_FOUND", err, nil))
			}
			tag, _ := c.Flags().GetString("tag")
			subject, _ := c.Flags().GetString("subject")

			type loreItem struct {
				ID      string   `json:"id"`
				Title   string   `json:"title"`
				Tags    []string `json:"tags,omitempty"`
				Subject string   `json:"subject,omitempty"`
			}

			// Collect matching ids in sorted order for stable output.
			ids := make([]string, 0, len(def.Lore))
			for id := range def.Lore {
				ids = append(ids, id)
			}
			sort.Strings(ids)

			var items []loreItem
			for _, id := range ids {
				entry := def.Lore[id]
				if tag != "" && !containsStrSlice(entry.Tags, tag) {
					continue
				}
				if subject != "" && entry.Subject != subject {
					continue
				}
				items = append(items, loreItem{
					ID:      id,
					Title:   entry.Title,
					Tags:    entry.Tags,
					Subject: entry.Subject,
				})
			}
			if items == nil {
				items = []loreItem{}
			}
			return a.emit(c, "lore.list", map[string]any{"lore": items}, nil)
		},
	}
	list.Flags().String("tag", "", "filter by tag")
	list.Flags().String("subject", "", "filter by subject id")

	// lore show <game> <id>
	show := &cobra.Command{
		Use:   "show <game> <id>",
		Short: "Show the full lore entry for <id>",
		Args:  cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			def, err := a.store().LoadDefinition(args[0])
			if err != nil {
				return a.emit(c, "lore.show", nil, coded("NOT_FOUND", err, nil))
			}
			entry, ok := def.Lore[args[1]]
			if !ok {
				return a.emit(c, "lore.show", nil, coded("NOT_FOUND",
					fmt.Errorf("lore entry %q not found", args[1]), nil))
			}
			return a.emit(c, "lore.show", map[string]any{
				"id":    args[1],
				"entry": entry,
			}, nil)
		},
	}

	cmd.AddCommand(list, show)
	return cmd
}

// containsStrSlice reports whether xs contains x.
func containsStrSlice(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

