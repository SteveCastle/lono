package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/callsignmedia/lono/internal/engine"
)

// canonicalKind maps alias kind names to their canonical kind. When a kind has
// no alias it maps to itself.
var canonicalKind = map[string]string{
	"item":              "item-type",
	"relationship-type": "rel-type",
	"event":             "beat",
	"branch":            "transition",
	// canonical forms (identity)
	"var":         "var",
	"entity-type": "entity-type",
	"item-type":   "item-type",
	"rel-type":    "rel-type",
	"machine":     "machine",
	"transition":  "transition",
	"derived":     "derived",
	"beat":        "beat",
	"trigger":     "trigger",
}

func (a *app) newDefineCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "define", Short: "Modify parts of a game definition"}
	for _, k := range []string{
		"var", "entity-type", "item-type", "rel-type", "machine", "transition", "derived", "beat", "trigger",
		// aliases
		"item", "relationship-type", "event", "branch",
	} {
		cmd.AddCommand(a.defineKind(k))
	}
	// scene is its own kind with two name args (machine + state).
	cmd.AddCommand(a.defineScene())
	return cmd
}

func (a *app) defineKind(kind string) *cobra.Command {
	canonical := canonicalKind[kind]
	if canonical == "" {
		canonical = kind
	}
	kc := &cobra.Command{Use: kind, Short: "Define " + kind}

	set := &cobra.Command{
		Use: "set <game> <name> [--spec|--spec-file]", Args: cobra.MinimumNArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			raw, err := readSpec(c)
			if err != nil {
				return a.emit(c, "define."+kind+".set", nil, coded("BAD_INPUT", err, nil))
			}
			return a.mutateDef(c, "define."+kind+".set", args[0], func(def *engine.Definition) error {
				return applyDefineSet(def, canonical, args[1:], raw)
			})
		},
	}
	addSpecFlags(set)

	rm := &cobra.Command{
		Use: "rm <game> <name>", Args: cobra.MinimumNArgs(2),
		RunE: func(c *cobra.Command, args []string) error {
			return a.mutateDef(c, "define."+kind+".rm", args[0], func(def *engine.Definition) error {
				return applyDefineRm(def, canonical, args[1:])
			})
		},
	}

	kc.AddCommand(set, rm)
	return kc
}

// defineScene returns the `scene` subcommand with its own set/rm children.
// Unlike regular kinds, scene takes TWO name args: <machine> <state>.
func (a *app) defineScene() *cobra.Command {
	sc := &cobra.Command{Use: "scene", Short: "Define scene metadata for a machine state"}

	set := &cobra.Command{
		Use:  "set <game> <machine> <state> [--spec|--spec-file]",
		Args: cobra.ExactArgs(3),
		RunE: func(c *cobra.Command, args []string) error {
			gameID, machine, state := args[0], args[1], args[2]
			raw, err := readSpec(c)
			if err != nil {
				return a.emit(c, "define.scene.set", nil, coded("BAD_INPUT", err, nil))
			}
			return a.mutateDef(c, "define.scene.set", gameID, func(def *engine.Definition) error {
				return applySceneSet(def, machine, state, raw)
			})
		},
	}
	addSpecFlags(set)

	rm := &cobra.Command{
		Use:  "rm <game> <machine> <state>",
		Args: cobra.ExactArgs(3),
		RunE: func(c *cobra.Command, args []string) error {
			gameID, machine, state := args[0], args[1], args[2]
			return a.mutateDef(c, "define.scene.rm", gameID, func(def *engine.Definition) error {
				return applySceneRm(def, machine, state)
			})
		},
	}

	sc.AddCommand(set, rm)
	return sc
}

// mutateDef loads, mutates, validates, and saves a definition.
func (a *app) mutateDef(c *cobra.Command, name, gameID string, fn func(*engine.Definition) error) error {
	def, err := a.store().LoadDefinition(gameID)
	if err != nil {
		return a.emit(c, name, nil, coded("NOT_FOUND", err, nil))
	}
	if err := fn(def); err != nil {
		return a.emit(c, name, nil, coded("BAD_INPUT", err, nil))
	}
	if errs := engine.ValidateDefinition(def); len(errs) > 0 {
		return a.emit(c, name, nil, coded("INVALID_DEFINITION", fmt.Errorf("definition would be invalid"), errs))
	}
	if err := a.store().SaveDefinition(def); err != nil {
		return a.emit(c, name, nil, coded("IO_ERROR", err, nil))
	}
	return a.emit(c, name, def, nil)
}

func ensureMaps(def *engine.Definition) {
	if def.World == nil {
		def.World = map[string]engine.VarSpec{}
	}
	if def.EntityTypes == nil {
		def.EntityTypes = map[string]engine.EntityType{}
	}
	if def.ItemTypes == nil {
		def.ItemTypes = map[string]engine.ItemType{}
	}
	if def.RelationshipTypes == nil {
		def.RelationshipTypes = map[string]engine.RelType{}
	}
	if def.Machines == nil {
		def.Machines = map[string]engine.Machine{}
	}
	if def.Derived == nil {
		def.Derived = map[string]engine.DerivedSpec{}
	}
	if def.Beats == nil {
		def.Beats = map[string]engine.Beat{}
	}
	if def.Triggers == nil {
		def.Triggers = map[string]engine.Trigger{}
	}
}

func applyDefineSet(def *engine.Definition, kind string, rest []string, raw []byte) error {
	ensureMaps(def)
	name := rest[0]
	switch kind {
	case "var":
		var s engine.VarSpec
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		def.World[name] = s
	case "entity-type":
		var t engine.EntityType
		if err := json.Unmarshal(raw, &t); err != nil {
			return err
		}
		def.EntityTypes[name] = t
	case "item-type":
		var t engine.ItemType
		if err := json.Unmarshal(raw, &t); err != nil {
			return err
		}
		def.ItemTypes[name] = t
	case "rel-type":
		var t engine.RelType
		if err := json.Unmarshal(raw, &t); err != nil {
			return err
		}
		def.RelationshipTypes[name] = t
	case "machine":
		var m engine.Machine
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		def.Machines[name] = m
	case "transition":
		m, ok := def.Machines[name] // here "name" is the machine name
		if !ok {
			return fmt.Errorf("unknown machine %q", name)
		}
		var tr engine.Transition
		if err := json.Unmarshal(raw, &tr); err != nil {
			return err
		}
		if tr.ID == "" {
			return fmt.Errorf("transition spec needs an id")
		}
		replaced := false
		for i := range m.Transitions {
			if m.Transitions[i].ID == tr.ID {
				m.Transitions[i] = tr
				replaced = true
				break
			}
		}
		if !replaced {
			m.Transitions = append(m.Transitions, tr)
		}
		def.Machines[name] = m
	case "derived":
		var d engine.DerivedSpec
		if err := json.Unmarshal(raw, &d); err != nil {
			return err
		}
		def.Derived[name] = d
	case "beat":
		var b engine.Beat
		if err := json.Unmarshal(raw, &b); err != nil {
			return err
		}
		def.Beats[name] = b
	case "trigger":
		var trig engine.Trigger
		if err := json.Unmarshal(raw, &trig); err != nil {
			return err
		}
		def.Triggers[name] = trig
	}
	return nil
}

// applySceneSet writes StateMeta for the given machine + state.
func applySceneSet(def *engine.Definition, machine, state string, raw []byte) error {
	ensureMaps(def)
	m, ok := def.Machines[machine]
	if !ok {
		return fmt.Errorf("unknown machine %q", machine)
	}
	var meta engine.StateMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return err
	}
	if m.StateMeta == nil {
		m.StateMeta = map[string]engine.StateMeta{}
	}
	m.StateMeta[state] = meta
	def.Machines[machine] = m
	return nil
}

// applySceneRm removes StateMeta for the given machine + state.
func applySceneRm(def *engine.Definition, machine, state string) error {
	ensureMaps(def)
	m, ok := def.Machines[machine]
	if !ok {
		return fmt.Errorf("unknown machine %q", machine)
	}
	if m.StateMeta != nil {
		delete(m.StateMeta, state)
	}
	def.Machines[machine] = m
	return nil
}

func applyDefineRm(def *engine.Definition, kind string, rest []string) error {
	ensureMaps(def)
	name := rest[0]
	switch kind {
	case "var":
		delete(def.World, name)
	case "entity-type":
		delete(def.EntityTypes, name)
	case "item-type":
		delete(def.ItemTypes, name)
	case "rel-type":
		delete(def.RelationshipTypes, name)
	case "machine":
		delete(def.Machines, name)
	case "derived":
		delete(def.Derived, name)
	case "beat":
		delete(def.Beats, name)
	case "transition":
		if len(rest) < 2 {
			return fmt.Errorf("usage: define transition rm <game> <machine> <transitionId>")
		}
		m, ok := def.Machines[name]
		if !ok {
			return fmt.Errorf("unknown machine %q", name)
		}
		out := m.Transitions[:0]
		for _, tr := range m.Transitions {
			if tr.ID != rest[1] {
				out = append(out, tr)
			}
		}
		m.Transitions = out
		def.Machines[name] = m
	case "trigger":
		delete(def.Triggers, name)
	}
	return nil
}
