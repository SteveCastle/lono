package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/callsignmedia/lono/internal/engine"
)

func (a *app) newGameCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "game", Short: "Author and manage game definitions"}

	create := &cobra.Command{
		Use: "create <id>", Short: "Create an empty game definition", Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			name, _ := c.Flags().GetString("name")
			if force, _ := c.Flags().GetBool("force"); !force {
				if _, err := a.store().LoadDefinition(args[0]); err == nil {
					return a.emit(c, "game.create", nil, coded("GAME_EXISTS",
						fmt.Errorf("game %q already exists; inspect it with `game show %s` and edit it with `define …` / `game export`, or pass --force to overwrite it with a blank definition", args[0], args[0]), nil))
				}
			}
			def := &engine.Definition{ID: args[0], Name: name, Version: 1}
			if err := a.store().SaveDefinition(def); err != nil {
				return a.emit(c, "game.create", nil, coded("IO_ERROR", err, nil))
			}
			return a.emit(c, "game.create", def, nil)
		},
	}
	create.Flags().String("name", "", "human-readable game name")
	create.Flags().Bool("force", false, "overwrite an existing game with a fresh empty definition")

	// list games (bare `game list`) — also parent for cast list subcommands.
	list := &cobra.Command{
		Use:   "list",
		Short: "List game ids (or cast members/relationships with a subcommand)",
		RunE: func(c *cobra.Command, args []string) error {
			ids, err := a.store().ListGames()
			if err != nil {
				return a.emit(c, "game.list", nil, coded("IO_ERROR", err, nil))
			}
			return a.emit(c, "game.list", map[string]any{"games": ids}, nil)
		},
	}

	show := &cobra.Command{
		Use: "show <id>", Short: "Show a game definition", Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			def, err := a.store().LoadDefinition(args[0])
			if err != nil {
				return a.emit(c, "game.show", nil, coded("NOT_FOUND", err, nil))
			}
			return a.emit(c, "game.show", def, nil)
		},
	}

	del := &cobra.Command{
		Use: "delete <id>", Short: "Delete a game definition", Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			if _, err := a.store().LoadDefinition(args[0]); err != nil {
				return a.emit(c, "game.delete", nil, coded("NOT_FOUND", err, nil))
			}
			if err := a.store().DeleteGame(args[0]); err != nil {
				return a.emit(c, "game.delete", nil, coded("IO_ERROR", err, nil))
			}
			return a.emit(c, "game.delete", map[string]string{"deleted": args[0]}, nil)
		},
	}

	validate := &cobra.Command{
		Use: "validate <id>", Short: "Validate a game definition", Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			def, err := a.store().LoadDefinition(args[0])
			if err != nil {
				return a.emit(c, "game.validate", nil, coded("NOT_FOUND", err, nil))
			}
			if errs := engine.ValidateDefinition(def); len(errs) > 0 {
				return a.emit(c, "game.validate", nil, coded("INVALID_DEFINITION", fmt.Errorf("%d problem(s)", len(errs)), errs))
			}
			return a.emit(c, "game.validate", map[string]bool{"valid": true}, nil)
		},
	}

	export := &cobra.Command{
		Use: "export <id>", Short: "Export a definition as JSON", Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			def, err := a.store().LoadDefinition(args[0])
			if err != nil {
				return a.emit(c, "game.export", nil, coded("NOT_FOUND", err, nil))
			}
			out, _ := c.Flags().GetString("out")
			if out != "" {
				b, _ := json.MarshalIndent(def, "", "  ")
				if err := os.WriteFile(out, b, 0o644); err != nil {
					return a.emit(c, "game.export", nil, coded("IO_ERROR", err, nil))
				}
				return a.emit(c, "game.export", map[string]string{"written": out}, nil)
			}
			return a.emit(c, "game.export", def, nil)
		},
	}
	export.Flags().StringP("out", "o", "", "write to file instead of stdout")

	imp := &cobra.Command{
		Use: "import", Short: "Import (and validate) a full definition",
		RunE: func(c *cobra.Command, args []string) error {
			raw, err := readSpec(c)
			if err != nil {
				return a.emit(c, "game.import", nil, coded("BAD_INPUT", err, nil))
			}
			var def engine.Definition
			if err := json.Unmarshal(raw, &def); err != nil {
				return a.emit(c, "game.import", nil, coded("BAD_INPUT", err, nil))
			}
			if errs := engine.ValidateDefinition(&def); len(errs) > 0 {
				return a.emit(c, "game.import", nil, coded("INVALID_DEFINITION", errors.New("definition is invalid"), errs))
			}
			if err := a.store().SaveDefinition(&def); err != nil {
				return a.emit(c, "game.import", nil, coded("IO_ERROR", err, nil))
			}
			return a.emit(c, "game.import", def, nil)
		},
	}
	addSpecFlags(imp)

	get := &cobra.Command{
		Use:   "get <game> [path]",
		Short: "Get a node from a game definition by path",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(c *cobra.Command, args []string) error {
			def, err := a.store().LoadDefinition(args[0])
			if err != nil {
				return a.emit(c, "game.get", nil, coded("NOT_FOUND", err, nil))
			}
			useTree, _ := c.Flags().GetBool("tree")
			depth, _ := c.Flags().GetInt("depth")
			if useTree {
				b, _ := json.Marshal(def)
				var m any
				_ = json.Unmarshal(b, &m)
				return a.emit(c, "game.get", map[string]any{"tree": engine.TreeView(m, depth)}, nil)
			}
			path := ""
			if len(args) == 2 {
				path = args[1]
			}
			v, err := engine.GetDefNode(def, path)
			if err != nil {
				if engine.IsNoSuchPath(err) {
					return a.emit(c, "game.get", nil, coded("NO_SUCH_PATH", err, nil))
				}
				return a.emit(c, "game.get", nil, coded("ERROR", err, nil))
			}
			return a.emit(c, "game.get", map[string]any{"path": path, "value": v}, nil)
		},
	}
	get.Flags().Bool("tree", false, "return names-only structural tree instead of a value")
	get.Flags().Int("depth", 2, "depth limit for --tree (default 2)")

	// -- cast commands --

	// `game add character <game> <id> --type <type> [--attrs '<json>']`
	// `game add relationship <game> <type> <from> <to> [--attrs '<json>']`
	addCmd := &cobra.Command{Use: "add", Short: "Add a cast member or relationship"}

	addCharacter := &cobra.Command{
		Use:   "character <game> <id> --type <type> [--attrs '<json>']",
		Short: "Add a character to the cast",
		Args:  cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			gameID, id := args[0], args[1]
			typ, _ := c.Flags().GetString("type")
			attrsRaw, _ := c.Flags().GetString("attrs")
			var attrs map[string]any
			if attrsRaw != "" {
				if err := json.Unmarshal([]byte(attrsRaw), &attrs); err != nil {
					return a.emit(c, "game.add.character", nil, coded("BAD_INPUT",
						fmt.Errorf("--attrs: %w", err), nil))
				}
			}
			return a.mutateDef(c, "game.add.character", gameID, func(def *engine.Definition) error {
				return engine.AddCharacter(def, id, typ, attrs)
			})
		},
	}
	addCharacter.Flags().String("type", "", "entity type for the character (required)")
	addCharacter.Flags().String("attrs", "", "initial attribute values as a JSON object")

	addRelationship := &cobra.Command{
		Use:   "relationship <game> <type> <from> <to> [--attrs '<json>']",
		Short: "Add a relationship between two cast members",
		Args:  cobra.ExactArgs(4),
		RunE: func(c *cobra.Command, args []string) error {
			gameID, typ, from, to := args[0], args[1], args[2], args[3]
			attrsRaw, _ := c.Flags().GetString("attrs")
			var attrs map[string]any
			if attrsRaw != "" {
				if err := json.Unmarshal([]byte(attrsRaw), &attrs); err != nil {
					return a.emit(c, "game.add.relationship", nil, coded("BAD_INPUT",
						fmt.Errorf("--attrs: %w", err), nil))
				}
			}
			return a.mutateDef(c, "game.add.relationship", gameID, func(def *engine.Definition) error {
				return engine.AddRelationship(def, typ, from, to, attrs)
			})
		},
	}
	addRelationship.Flags().String("attrs", "", "relationship attribute values as a JSON object")

	addCmd.AddCommand(addCharacter, addRelationship)

	// `game give <game> <character> --item <item> [--count N] [--equip <slot>]`
	give := &cobra.Command{
		Use:   "give <game> <character> --item <item> [--count N] [--equip <slot>]",
		Short: "Give an item to a cast member",
		Args:  cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			gameID, charID := args[0], args[1]
			item, _ := c.Flags().GetString("item")
			count, _ := c.Flags().GetInt("count")
			equipSlot, _ := c.Flags().GetString("equip")
			if item == "" {
				return a.emit(c, "game.give", nil, coded("BAD_INPUT",
					fmt.Errorf("--item is required"), nil))
			}
			// Default count to 1 when no --count supplied and no --equip-only scenario.
			if count == 0 && equipSlot == "" {
				count = 1
			}
			return a.mutateDef(c, "game.give", gameID, func(def *engine.Definition) error {
				return engine.GiveItem(def, charID, item, count, equipSlot)
			})
		},
	}
	give.Flags().String("item", "", "item type to give (required)")
	give.Flags().Int("count", 0, "number of items to add (default 1 when --equip not set)")
	give.Flags().String("equip", "", "equip the item in this slot")

	// `game rm character <game> <id>`
	// `game rm relationship <game> <type> <from> <to>`
	rmCmd := &cobra.Command{Use: "rm", Short: "Remove a cast member or relationship"}

	rmCharacter := &cobra.Command{
		Use:   "character <game> <id>",
		Short: "Remove a character from the cast",
		Args:  cobra.ExactArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			gameID, id := args[0], args[1]
			return a.mutateDef(c, "game.rm.character", gameID, func(def *engine.Definition) error {
				return engine.RemoveCharacter(def, id)
			})
		},
	}

	rmRelationship := &cobra.Command{
		Use:   "relationship <game> <type> <from> <to>",
		Short: "Remove a relationship from the cast",
		Args:  cobra.ExactArgs(4),
		RunE: func(c *cobra.Command, args []string) error {
			gameID, typ, from, to := args[0], args[1], args[2], args[3]
			return a.mutateDef(c, "game.rm.relationship", gameID, func(def *engine.Definition) error {
				return engine.RemoveRelationship(def, typ, from, to)
			})
		},
	}

	rmCmd.AddCommand(rmCharacter, rmRelationship)

	// `game list characters <game>` / `game list relationships <game>`
	// These are subcommands of the existing `list` command.
	listCharacters := &cobra.Command{
		Use:   "characters <game>",
		Short: "List characters in the cast",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			def, err := a.store().LoadDefinition(args[0])
			if err != nil {
				return a.emit(c, "game.list.characters", nil, coded("NOT_FOUND", err, nil))
			}
			entities := def.Entities
			if entities == nil {
				entities = map[string]engine.EntityInit{}
			}
			return a.emit(c, "game.list.characters",
				map[string]any{"characters": entities}, nil)
		},
	}

	listRelationships := &cobra.Command{
		Use:   "relationships <game>",
		Short: "List relationships in the cast",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			def, err := a.store().LoadDefinition(args[0])
			if err != nil {
				return a.emit(c, "game.list.relationships", nil, coded("NOT_FOUND", err, nil))
			}
			rels := def.Relationships
			if rels == nil {
				rels = []engine.RelInit{}
			}
			return a.emit(c, "game.list.relationships",
				map[string]any{"relationships": rels}, nil)
		},
	}

	list.AddCommand(listCharacters, listRelationships)

	cmd.AddCommand(create, list, show, del, validate, export, imp, get,
		addCmd, give, rmCmd)
	return cmd
}
