# lono authoring reference

The complete vocabulary for building a game definition. All `--spec` payloads are
JSON. Run every command with the same `--data-dir` (shown as `-d` below for
brevity; use the full `--data-dir ./.lono`).

## Authoring commands

```
lono game create <id> --name "<Title>" [--force]   # refuses to overwrite an existing game unless --force
lono game validate <id>                 # ALWAYS run after changes; fix ok:false
lono game show <id> [--pretty]          # introspect the FULL current definition before editing
lono game export <id> -o <file>         # save the portable definition
lono game import --spec-file <file>     # load a whole definition (validated)
lono game import --spec '<json>'        # or inline; replaces the game by its id
lono game list | delete <id>

# Types / world / rules
lono define var               set|rm <game> <name> --spec '<VarSpec>'
lono define entity-type       set|rm <game> <name> --spec '<EntityType>'
lono define item              set|rm <game> <name> --spec '<ItemType>'       # alias: item-type
lono define relationship-type set|rm <game> <name> --spec '<RelType>'        # alias: rel-type
lono define machine           set|rm <game> <name> --spec '<Machine>'
lono define scene             set|rm <game> <machine> <state> --spec '<StateMeta>'  # one state's metadata only
lono define branch            set|rm <game> <machine> --spec '<Transition>'  # alias: transition; id is in the spec
lono define derived           set|rm <game> <name> --spec '<DerivedSpec>'
lono define event             set|rm <game> <name> --spec '<Beat>'           # alias: beat
lono define trigger           set|rm <game> <name> --spec '<Trigger>'       # reactive rule (fires automatically)
lono define <kind>            rm  <game> <name>     # branch/transition: rm <game> <machine> <transitionId>
```

`game import` is often easiest for a big definition: build the whole JSON object
and import it once (it validates atomically). The `define …` commands are for
incremental edits.

## Construction commands (cast)

Use these to build the concrete starting cast directly into the definition's
first-class `entities` and `relationships` sections. They re-validate the whole
definition after each change and are the preferred way to add characters and
links (over hand-writing `setup` ops).

```
lono game add character    <game> <id> --type <type> [--attrs '<json>']
lono game add relationship <game> <type> <from> <to> [--attrs '<json>']
lono game give             <game> <character> --item <item> [--count N] [--equip <slot>]
lono game rm  character    <game> <id>
lono game rm  relationship <game> <type> <from> <to>
lono game list characters  <game>
lono game list relationships <game>
```

These populate two first-class definition sections that `StartInstance` seeds
before running `setup`:

```json
"entities": {
  "aria": {
    "type": "character",
    "attrs": {"name": "Aria", "mood": "curious"},
    "inventory": {"rose": 1},
    "equipped": {"torso": "silk_dress"}
  },
  "player": {"type": "character", "attrs": {"name": "Vex"}}
}
```

```json
"relationships": [
  {"type": "romance", "from": "aria", "to": "player", "attrs": {"affection": 0, "trust": 10}}
]
```

Validation rules: entity `type` must be a defined entity-type; attrs, inventory
items, and equipped slots are validated against the type definition. Relationship
`type` must be defined; `from`/`to` must be ids present in `entities`.

Raw `setup` effect ops remain supported and run **after** the declarative cast
(use them for scripted seeding: rolls, conditional logic, anything not
expressible declaratively).

## Introspecting (get / tree)

Navigate the definition tree to the exact node you need — faster than reading
the whole `game show` dump:

```
lono game get <game> [path] [--tree] [--depth N] [--pretty]
```

- No path → returns the whole definition (same as `game show`).
- `path` → returns the JSON value at that `/`-delimited node.
- `--tree` → returns a names-only structural map (good for orientation).
- `--depth N` → limits tree depth.

Path examples:

```
lono game get heist machines/arc/transitions/begin    # one transition by id
lono game get heist entities/aria                     # aria's starting cast entry
lono game get heist machines/arc/stateMeta/escaped_clean
lono game get heist --tree                            # structural overview
lono game get heist entityTypes/character/attributes
```

Unresolved paths return a `NO_SUCH_PATH` error. Arrays of objects with an `id`
field (transitions) are addressed by id; `setup` and `relationships` by index.

## Value types (VarSpec)

Used for world vars, entity/relationship attributes, and action params.

```json
{"type": "int",   "default": 0, "min": 0, "max": 100}
{"type": "float", "default": 0.0}
{"type": "bool",  "default": false}
{"type": "string","default": ""}
{"type": "enum",  "values": ["sun","rain","storm"], "default": "sun"}
{"type": "ref",   "refType": "location"}        // points to an entity of that type
{"type": "set",   "elem": "string"}             // a set of unique strings, default []
{"type": "set",   "elem": "ref", "refType": "character"}  // a set of entity ids
```
Numbers are JSON numbers. `int` rejects non-whole values. `enum` default must be
a listed value. `ref` value is an entity id (validated to exist). A `set` is
stored as a JSON array with unique membership (default `[]`); for `elem:"ref"`
each member must be an existing entity. Mutate sets with `add_to`/`remove_from`/
`clear`, test with the `contains` guard, and size with `len.<path>`.

## Definition shape (what `game import` accepts)

```json
{
  "id": "heist", "name": "The Midnight Heist", "version": 1,
  "description": "A noir one-night heist.", "intent": "tense, morally grey",

  "world": { "alarm": {"type":"bool","default":false},
             "loot":  {"type":"int","default":0,"min":0} },

  "entityTypes": {
    "character": {"description":"a person",
      "attributes": {"name":{"type":"string"},
                     "health":{"type":"int","default":100,"min":0,"max":100},
                     "location":{"type":"ref","refType":"location"}},
      "slots": {"torso":{"accepts":["dress","suit"]}}},
    "location": {"attributes": {"name":{"type":"string"}}}
  },

  "itemTypes": {
    "gold":       {"maxStack": 100000},
    "silk_dress": {"category":"dress","equippable":true,
                   "attributes":{"style":8,"warmth":2,"color":"midnight-blue"}}
  },

  "relationshipTypes": {
    "romance": {"from":"character","to":"character","directed":true,
      "attributes":{"affection":{"type":"int","default":0,"min":-100,"max":100},
                    "trust":{"type":"int","default":0,"min":-100,"max":100}}}
  },

  "derived": { "...": "see Derived values" },
  "machines": { "...": "see State machines" },
  "beats":    { "...": "see Beats" },

  "entities": {
    "player": {"type":"character","attrs":{"name":"Vex"}},
    "aria":   {"type":"character","attrs":{"name":"Aria"},"inventory":{"rose":1},"equipped":{"torso":"silk_dress"}}
  },
  "relationships": [
    {"type":"romance","from":"aria","to":"player","attrs":{"affection":0,"trust":10}}
  ],

  "setup": [ "...optional advanced effect ops run after the declarative cast..." ]
}
```
Item-type `attributes` are **static per type** (plain values, not VarSpecs) — all
`silk_dress` share them. `category` + `equippable` enable wearing into a slot that
`accepts` that category.

## State machines

A machine is a set of `states`, an `initial`, optional per-state narrative in
`stateMeta`, and `transitions` (the actions).

```json
"arc": {
  "initial": "outside",
  "states": ["outside","inside","escaped_clean","caught"],
  "stateMeta": {
    "escaped_clean": {"terminal":true,"ending":true,
       "description":"You vanish into the night, loot in hand and trust intact.",
       "intent":"reached when loot>0 and the alarm never sounded"},
    "caught": {"terminal":true,"ending":true,"description":"The cuffs click shut."}
  },
  "transitions": [
    {"id":"break_in","from":"outside","to":"inside",
     "description":"You jimmy the vault door.",
     "guard":{"target":"inventory.player.lockpick","op":"gt","value":0},
     "effects":[{"op":"remove_item","entity":"player","item":"lockpick","count":1},
                {"op":"set","target":"entity.player.location","value":"vault"}]},
    {"id":"escape","from":"inside","to":"escaped_clean",
     "guard":{"and":[{"target":"world.loot","op":"gt","value":0},
                     {"target":"world.alarm","op":"eq","value":false}]}}
  ]
}
```
- `from` may be a single state `"x"`, a list `["x","y"]`, or `"*"` (any). An
  internal action has `from == to`.
- A transition with `params` (a `{name: VarSpec}` map) takes runtime arguments,
  readable in its guard/effects as `param.<name>`.
- **Endings** = terminal states. When a machine is in a `terminal` state the
  engine reports it under `endingReached`. Author 2–4 endings as terminal states
  with prose + intent, reached by guarded transitions.

### Attached machines (per-couple / per-character arcs)

Add `attach` to instantiate the machine **per host** (each relationship or
entity of a type carries its own state). Transitions reference the host via
`this.*`.

```json
"romance_stage": {
  "attach": {"to": "relationshipType:romance"},   // or "entityType:character"
  "initial": "strangers",
  "states": ["strangers","friends","dating","partners","exes"],
  "transitions": [
    {"id":"start_dating","from":"friends","to":"dating",
     "guard":{"target":"this.affection","op":"gte","value":60},
     "effects":[{"op":"inc","target":"this.affection","value":5}]}]
}
```
At runtime these are performed per host: `do <run> romance_stage start_dating --rel aria,player`
(relationship host) or `--entity <id>` (entity host). Beats may **not** bind to an
attached machine (use a guard on `rel.*`/`entity.*` instead).

## Guards (when an action / beat is allowed)

A guard is a tree. Leaf: `{"target":<path>,"op":<op>,"value":<v>}`. Combinators:
`{"and":[...]}`, `{"or":[...]}`, `{"not":{...}}`. A missing guard is always true.

Leaf ops: `eq ne gt gte lt lte in exists contains`.
- `in`: value is a list; true if target ∈ list.
- `exists`: true if the path refers to something present (entity/relationship
  present; inventory/equipped slot non-empty/non-zero). Use `gt 0` for "has item".
- `contains`: target is a `set` path; true if the set contains `value`, e.g.
  `{"target":"entity.player.party","op":"contains","value":"aria"}`.

## Dotted paths (read in guards, derived, and effect targets)

| Path | Reads |
|------|-------|
| `world.<var>` | a world variable |
| `entity.<id>.<attr>` | an entity attribute |
| `entity.<id>.derived.<name>` | a per-entity derived value (`$self` derived) |
| `inventory.<id>.<item>` | item count (0 if none) |
| `equipped.<id>.<slot>` | item type worn in a slot (`""` if empty) |
| `worn.<id>.<slot>.<attr>` | attribute of the item in a slot (errors if empty) |
| `itemtype.<id>.<attr>` | a static item-type attribute |
| `rel.<type>.<from>.<to>.<attr>` | a relationship attribute |
| `machine.<name>.state` | a global machine's current state |
| `derived.<name>` | a global derived value |
| `len.<path>` | the length of the set/array at `<path>` (e.g. `len.entity.player.clues`) |
| `clock` | the instance clock — an integer tick advanced by `advance` |
| `roll.<store>` | a roll stored earlier this action (readable in guards for skill checks) |
| `cooldown.<key>` | ticks remaining on a cooldown, `max(0, due-clock)`; `0` = ready |
| `param.<name>` | an action parameter (inside that action only) |
| `this.<attr>` / `this.from[.attr]` / `this.to[.attr]` / `this.id` / `this.inventory.<item>` / `this.equipped.<slot>` / `this.machine.<name>` | the host of an attached-machine transition |

## Effect ops (what a transition / setup / apply changes)

Scalar (target is `world.<v>`, `entity.<id>.<attr>`, or `this.<attr>`; bounds enforced):
```json
{"op":"set","target":"world.alarm","value":true}
{"op":"inc","target":"entity.player.health","value":10}     // also dec, mul
{"op":"set","target":"entity.player.health","value":{"$roll":"dmg"}}  // use a roll result
{"op":"inc","target":"world.gold","value":{"$path":"world.day"}}      // value from a path
```
Any operand (a `set`/`inc`/`dec`/`mul` value, or a `compute` operand) may be a
literal, `{"$roll":"<store>"}` (a roll result), or `{"$path":"<path>"}` (the
current value at a path).

Collections (target is a `set` path): `{"op":"add_to","target":"entity.player.clues","value":"ledger"}` (no-op if present) · `{"op":"remove_from","target":"entity.player.party","value":"aria"}` (no-op if absent) · `{"op":"clear","target":"entity.player.clues"}`.
Compute: `{"op":"compute","target":"entity.goblin.health","fn":"sub","a":{"$path":"entity.goblin.health"},"b":{"$path":"entity.player.strength"}}` — sets `target = a fn b`, `fn ∈ add|sub|mul|div|min|max|mod` (result bounds-checked against the target).
Conditional: `{"op":"if","when":<Guard>,"then":[…],"else":[…]}` — applies `then` or `else` (each a normal effect list; `else` optional; nestable).
Schedule (delayed effects / deadlines): `{"op":"schedule","in":N,"do":[…]}` — enqueues the effects to run `N` ticks from now (fired by `advance`).
Cooldown: `{"op":"cooldown","key":"confess","ticks":2}` — gates re-use; pair with a guard `{"target":"cooldown.confess","op":"eq","value":0}`.
Journal: `{"op":"record","text":"Aria forgave you.","tags":["aria"]}` — appends a narrative entry (see Narrative journal).
Inventory: `{"op":"add_item","entity":"player","item":"gold","count":50}` (also `remove_item`; respects `maxStack` / non-negative).
Entities: `{"op":"create_entity","entityType":"character","id":"aria","attrs":{"name":"Aria"}}` · `{"op":"destroy_entity","id":"aria"}`.
Relationships: `{"op":"set_relationship","relType":"romance","from":"aria","to":"player","attrs":{"affection":0}}` · `{"op":"adjust_relationship",...,"attr":"affection","by":5}` · `{"op":"remove_relationship",...}`.
Machines: `{"op":"set_machine_state","machine":"arc","state":"caught"}` (global) · `{"op":"set_attached_state","machine":"romance_stage","from":"aria","to":"player","state":"dating"}` (attached; use `"entity":"<id>"` for entity-attached).
Dice: `{"op":"roll","dice":"1d6","store":"dmg"}` then reference `{"$roll":"dmg"}` in a later op (deterministic per instance seed; supports `NdM`, `NdM+K`, `NdM-K`).
Narrative: `{"op":"mark_beat","beat":"aria_first_smile"}` (records a one-shot beat as delivered).
Equipment: `{"op":"equip","entity":"player","slot":"torso","item":"silk_dress"}` · `{"op":"unequip","entity":"player","slot":"torso"}` (slot must exist & accept the item's category; one item per slot).

## Derived values (reusable social-graph queries)

Computed on read; usable anywhere a path is.

```json
"derived": {
  "player_admirers": {"over":"relationships",
    "where":{"type":"romance","to":"player","attrs":[{"attr":"affection","op":"gte","value":80}]},
    "reduce":"count","intent":"how many NPCs strongly love the player"},
  "top_admirer": {"over":"relationships","where":{"type":"romance","to":"player"},
    "reduce":"argmax:affection"},
  "my_friend_count": {"over":"relationships","where":{"type":"friendship","from":"$self"},
    "reduce":"count"}
}
```
- `over`: `relationships` | `entities`. `where`: `type`, `from`/`to` (relationship
  endpoints — a literal id or `$self`), and `attrs` predicates.
- `reduce`: `count` `any` `sum:<attr>` `min:<attr>` `max:<attr>` `argmax:<attr>`
  `argmin:<attr>`. `argmax`/`argmin` over relationships return the matching edge's
  `from` id (anchor on `to` to get "who…"); over entities, the entity id.
- A derived whose `where` uses `$self` is **per-entity** — read it as
  `entity.<id>.derived.<name>`. Others are global: `derived.<name>`.

## Beats (authored narrative the engine surfaces)

```json
"beats": {
  "aria_first_smile": {
    "text": "Aria looks up from her drink and, for the first time, smiles at you.",
    "machineState": {"machine":"arc","state":"bar"},
    "guard": {"target":"rel.romance.aria.player.affection","op":"gte","value":20},
    "once": true,
    "intent": "the moment Aria first warms to the player"
  }
}
```
A beat is **active** when its `machineState` matches the current state (global
machines only; optional), its `guard` holds (optional; may reference derived),
and — if `once` (the default) — it hasn't been delivered. The runtime lists
active beats; deliver one with the `mark_beat` op so once-beats stop repeating.
Set `"once": false` for ambient/repeatable beats.

## Reactive rules (triggers) & time

A **trigger** is a rule the engine fires **automatically** — you don't invoke it.
Define them on the game with `define trigger set <game> <name> --spec '<Trigger>'`:

```json
{"when":{"target":"world.alarm","op":"eq","value":true},
 "effects":[{"op":"set","target":"entity.guard.hostile","value":true},
            {"op":"schedule","in":3,"do":[{"op":"set_machine_state","machine":"arc","state":"caught"}]}],
 "once":true,
 "intent":"the alarm turns the guard hostile and starts a 3-turn capture clock"}
```
`Trigger{ when?: Guard, every?: int, once?: bool, effects: [Effect], intent: string }`
(must have a `when` and/or `every`).

- **Reactive (`when`).** After **every** committed change (`do`, `apply`, runtime
  `set`/`rm`, and each `advance` tick) the engine settles: it fires triggers whose
  `when` guard just became true. Firing is **edge-triggered** (fires on the rising
  edge, re-arms when the guard goes false again) so it can't loop; cascades (one
  trigger making another's guard true) resolve in the same settle.
- **`once:true`** fires at most once ever (recommended for one-shot consequences);
  `once:false` re-fires on each rising edge.
- **Periodic (`every:N`).** Fires every `N` ticks of the clock — only during
  `advance`, not on ordinary changes.

**Time.** Each instance has an integer `clock` (path `clock`). The runtime
`advance <instance> [n]` command ticks it: per tick it runs any due `schedule`d
effects, fires periodic triggers, then settles. Use `schedule` for deadlines/
delayed consequences and `cooldown` to gate re-use of an action over time.

Encode automatic consequences and timers as triggers + `schedule`/`cooldown` so
the engine enforces them — don't rely on the narrator to remember to apply them.

## Narrative journal

The engine keeps an LLM- and trigger-authored log of what happened (separate from
the mechanical history). Append to it with the `record` op:

```json
{"op":"record","text":"Aria forgave you and returned the locket.","tags":["aria","day2"]}
```
Use it (in trigger/transition `effects`, or at runtime via `apply`) at
consequential moments so any session — including a resumed one — can recall the
story. Recent entries show up in `state` output; the full journal is read with
`inspect <run> log`.

## Skill checks (roll → if → compute)

A single action can express a dice check by storing a roll and branching on the
`roll.<store>` path:

```json
[{"op":"roll","dice":"1d20","store":"atk"},
 {"op":"if","when":{"target":"roll.atk","op":"gte","value":12},
    "then":[{"op":"compute","target":"entity.goblin.health","fn":"sub",
             "a":{"$path":"entity.goblin.health"},"b":{"$roll":"atk"}}],
    "else":[{"op":"mark_beat","beat":"you_miss"}]}]
```
Put quantitative outcomes (damage, scaled costs, checks) in `compute`/`if`/`$path`/
`roll.<store>` so the engine does the math — never ask the model to compute it.

## Setup (seed the starting state)

A `setup` list of effect ops runs once when an instance starts, so a fresh game
self-populates its entities/relationships/inventory/worn items:

```json
"setup": [
  {"op":"create_entity","entityType":"location","id":"vault","attrs":{"name":"The Vault"}},
  {"op":"create_entity","entityType":"character","id":"player","attrs":{"name":"Vex"}},
  {"op":"create_entity","entityType":"character","id":"aria","attrs":{"name":"Aria"}},
  {"op":"add_item","entity":"player","item":"lockpick","count":2},
  {"op":"set_relationship","relType":"romance","from":"aria","to":"player","attrs":{"affection":0}}
]
```
Attached machines initialize automatically when their host is created here.

## Validation errors you'll hit

`game validate` (and `import`) reject: unknown types/states referenced; enum
defaults not in `values`; numeric defaults out of bounds; machine `initial`/
transition `to`/`from` states not in `states`; relationship `from`/`to` not a
defined entity type; `attach.to` naming an unknown type; derived `over`/`reduce`
invalid or referencing an attribute the type doesn't define; a beat bound to an
unknown or attached machine, or with empty text. Read the `error.details` list —
each entry has a `path` and `message`.
