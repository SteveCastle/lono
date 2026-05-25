package editor

// This file is the single source of truth the editor UI uses to build its
// dropdowns and per-op field forms. It mirrors the vocabulary enforced by the
// engine (guards.go / effects.go / validate.go); keep it in sync when the engine
// gains ops, types, or operators.

// FieldSpec describes one input the UI should render for an effect op.
// Kind drives the widget:
//
//	string  - text input
//	int     - integer input
//	value   - arbitrary JSON value (string/number/bool)
//	attrs   - map of attr -> JSON value
//	tags    - list of strings
//	guard   - a nested guard editor
//	effects - a nested list of effects (if/then/else, schedule/do)
type FieldSpec struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Doc  string `json:"doc,omitempty"`
}

// EffectOpSpec is one selectable effect op plus the fields it uses.
type EffectOpSpec struct {
	Op     string      `json:"op"`
	Group  string      `json:"group"`
	Doc    string      `json:"doc"`
	Fields []FieldSpec `json:"fields"`
}

// Meta is the payload served at GET /api/meta — everything the UI needs to know
// about the engine's vocabulary, plus the working directory.
type Meta struct {
	Version     string         `json:"version"`
	Dir         string         `json:"dir"`
	VarTypes    []string       `json:"varTypes"`
	SetElems    []string       `json:"setElems"`
	GuardOps    []string       `json:"guardOps"`
	ReduceVerbs []string       `json:"reduceVerbs"`
	ComputeFns  []string       `json:"computeFns"`
	AttachKinds []string       `json:"attachKinds"`
	DerivedOver []string       `json:"derivedOver"`
	EffectOps   []EffectOpSpec `json:"effectOps"`
}

func buildMeta(dir string) Meta {
	return Meta{
		Version:     editorVersion,
		Dir:         dir,
		VarTypes:    []string{"int", "float", "bool", "string", "ref", "enum", "set"},
		SetElems:    []string{"string", "ref"},
		GuardOps:    []string{"eq", "ne", "gt", "gte", "lt", "lte", "in", "contains", "exists"},
		ReduceVerbs: []string{"count", "any", "list", "sum:<attr>", "min:<attr>", "max:<attr>", "argmax:<attr>", "argmin:<attr>"},
		ComputeFns:  []string{"add", "sub", "mul", "div", "min", "max", "mod"},
		AttachKinds: []string{"entityType:<name>", "relationshipType:<name>"},
		DerivedOver: []string{"entities", "relationships"},
		EffectOps:   effectOpSpecs,
	}
}

// effectOpSpecs enumerates every effect op the engine understands and the fields
// each one reads, grouped for a designer-friendly menu.
var effectOpSpecs = []EffectOpSpec{
	// --- state ---
	{Op: "set", Group: "State", Doc: "Set a world var or entity attribute to a value.",
		Fields: []FieldSpec{{"target", "string", "path, e.g. world.alarm or entity.player.health"}, {"value", "value", ""}}},
	{Op: "inc", Group: "State", Doc: "Increase a numeric path by value.",
		Fields: []FieldSpec{{"target", "string", ""}, {"value", "value", "amount"}}},
	{Op: "dec", Group: "State", Doc: "Decrease a numeric path by value.",
		Fields: []FieldSpec{{"target", "string", ""}, {"value", "value", "amount"}}},
	{Op: "mul", Group: "State", Doc: "Multiply a numeric path by value.",
		Fields: []FieldSpec{{"target", "string", ""}, {"value", "value", "factor"}}},
	{Op: "compute", Group: "State", Doc: "Compute target = fn(a, b). Operands may be literals or {\"$path\":\"...\"}.",
		Fields: []FieldSpec{{"target", "string", ""}, {"fn", "string", "add|sub|mul|div|min|max|mod"}, {"a", "value", ""}, {"b", "value", ""}}},

	// --- sets ---
	{Op: "add_to", Group: "Sets", Doc: "Add a member to a set attribute.",
		Fields: []FieldSpec{{"target", "string", "a set path"}, {"value", "value", "member"}}},
	{Op: "remove_from", Group: "Sets", Doc: "Remove a member from a set attribute.",
		Fields: []FieldSpec{{"target", "string", "a set path"}, {"value", "value", "member"}}},
	{Op: "clear", Group: "Sets", Doc: "Empty a set attribute.",
		Fields: []FieldSpec{{"target", "string", "a set path"}}},

	// --- items / equipment ---
	{Op: "add_item", Group: "Items", Doc: "Add count of an item type to an entity's inventory.",
		Fields: []FieldSpec{{"entity", "string", ""}, {"item", "string", "item type id"}, {"count", "int", ""}}},
	{Op: "remove_item", Group: "Items", Doc: "Remove count of an item type from an entity's inventory.",
		Fields: []FieldSpec{{"entity", "string", ""}, {"item", "string", "item type id"}, {"count", "int", ""}}},
	{Op: "equip", Group: "Items", Doc: "Equip an item into a slot on an entity.",
		Fields: []FieldSpec{{"entity", "string", ""}, {"slot", "string", ""}, {"item", "string", "item type id"}}},
	{Op: "unequip", Group: "Items", Doc: "Clear an entity's equipment slot.",
		Fields: []FieldSpec{{"entity", "string", ""}, {"slot", "string", ""}}},

	// --- entities / relationships ---
	{Op: "create_entity", Group: "Entities", Doc: "Create a new entity instance at runtime.",
		Fields: []FieldSpec{{"entityType", "string", ""}, {"id", "string", "new entity id"}, {"attrs", "attrs", ""}}},
	{Op: "destroy_entity", Group: "Entities", Doc: "Remove an entity instance.",
		Fields: []FieldSpec{{"id", "string", "entity id"}}},
	{Op: "move", Group: "Entities", Doc: "Move an entity along a ref attr; via requires a connecting relationship (an exit).",
		Fields: []FieldSpec{{"entity", "string", ""}, {"to", "string", "destination entity id"}, {"attr", "string", "ref attr (default location)"}, {"via", "string", "relationship type, e.g. exit"}}},
	{Op: "set_relationship", Group: "Relationships", Doc: "Create or replace a relationship's attrs.",
		Fields: []FieldSpec{{"relType", "string", ""}, {"from", "string", ""}, {"to", "string", ""}, {"attrs", "attrs", ""}}},
	{Op: "adjust_relationship", Group: "Relationships", Doc: "Add `by` to one relationship attribute.",
		Fields: []FieldSpec{{"relType", "string", ""}, {"from", "string", ""}, {"to", "string", ""}, {"attr", "string", ""}, {"by", "value", "delta (number)"}}},
	{Op: "remove_relationship", Group: "Relationships", Doc: "Delete a relationship.",
		Fields: []FieldSpec{{"relType", "string", ""}, {"from", "string", ""}, {"to", "string", ""}}},

	// --- machines ---
	{Op: "set_machine_state", Group: "Machines", Doc: "Force a global machine into a state.",
		Fields: []FieldSpec{{"machine", "string", ""}, {"state", "string", ""}}},
	{Op: "set_attached_state", Group: "Machines", Doc: "Set an attached machine's state on a host (entity, or from/to for a relationship).",
		Fields: []FieldSpec{{"machine", "string", ""}, {"state", "string", ""}, {"entity", "string", "host entity (entityType machines)"}, {"from", "string", "host from (relationshipType machines)"}, {"to", "string", "host to"}}},

	// --- dice ---
	{Op: "roll", Group: "Dice", Doc: "Roll dice (e.g. 1d20) and store the result under roll.<store>.",
		Fields: []FieldSpec{{"dice", "string", "e.g. 1d20, 2d6+1"}, {"store", "string", "key, read as roll.<store>"}}},
	{Op: "check", Group: "Dice", Doc: "Skill check: roll dice + modifiers vs a DC. Branch on check.<store>.success / .margin / .total / .roll.",
		Fields: []FieldSpec{{"dice", "string", "e.g. 1d20"}, {"mods", "values", "modifiers: numbers or {\"$path\":\"entity.x.skill\"}"}, {"dc", "value", "target number, or {\"$path\":\"…\"} for an opposed check"}, {"store", "string", "key, read as check.<store>.*"}}},

	// --- control flow ---
	{Op: "if", Group: "Control flow", Doc: "Run `then` effects if `when` holds, else `else`.",
		Fields: []FieldSpec{{"when", "guard", ""}, {"then", "effects", ""}, {"else", "effects", ""}}},
	{Op: "schedule", Group: "Control flow", Doc: "Enqueue `do` effects to fire in `in` ticks (via advance).",
		Fields: []FieldSpec{{"in", "int", "ticks from now"}, {"do", "effects", ""}}},
	{Op: "cooldown", Group: "Control flow", Doc: "Set a named cooldown expiring in `ticks` ticks (read as cooldown.<key>).",
		Fields: []FieldSpec{{"key", "string", ""}, {"ticks", "int", ""}}},

	// --- narrative ---
	{Op: "mark_beat", Group: "Narrative", Doc: "Mark a one-shot beat as delivered.",
		Fields: []FieldSpec{{"beat", "string", "beat id"}}},
	{Op: "record", Group: "Narrative", Doc: "Append an entry to the narrative journal.",
		Fields: []FieldSpec{{"text", "string", ""}, {"tags", "tags", ""}}},
	{Op: "discover", Group: "Narrative", Doc: "Reveal a lore entry (adds it to discoveredLore).",
		Fields: []FieldSpec{{"lore", "string", "lore id"}}},
}

// starterTemplate returns a minimal valid definition for a brand-new file.
func starterTemplate(id string) map[string]any {
	return map[string]any{
		"id":          id,
		"name":        id,
		"description": "",
		"intent":      "",
		"version":     1,
		"world":       map[string]any{},
		"entityTypes": map[string]any{
			"character": map[string]any{
				"description": "A person in the story",
				"attributes": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
		"machines": map[string]any{
			"arc": map[string]any{
				"description": "The main story arc",
				"initial":     "start",
				"states":      []string{"start", "end"},
				"stateMeta": map[string]any{
					"end": map[string]any{"description": "The story concludes.", "terminal": true, "ending": true},
				},
				"transitions": []any{
					map[string]any{"id": "finish", "from": "start", "to": "end"},
				},
			},
		},
		"entities": map[string]any{
			"player": map[string]any{
				"type":        "character",
				"attrs":       map[string]any{"name": "You"},
				"description": "The protagonist.",
			},
		},
	}
}

const editorVersion = "0.5.0"
