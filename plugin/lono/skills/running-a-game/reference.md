# lono runtime reference

Commands and output format for playing a game. Use one `--data-dir` for the whole
session (shown abbreviated; use the full `--data-dir ./.lono`).

## Runtime commands

```
lono game import --spec-file <file>            # load a saved definition (validated)
lono game list                                 # existing game ids
lono play start <game> --id <run> [--seed <n>] # begin a session -> opening state+actions
lono play list                                 # existing instance ids
lono state <run> [--pretty]                    # KEY READ: full state + actions + beats + endings
lono actions <run>                             # just the available actions
lono do <run> <machine> <action> [--params '<json>'] [--entity <id> | --rel <from>,<to>]
lono advance <run> [n]                         # advance in-game time n ticks (default 1): fires scheduled/periodic/reactive
lono apply <run> --ops '<json-array>'          # ad-hoc state updates (documented ops only)
lono inspect <run> [path] [--tree] [--depth N] # targeted read of live state (see §Inspect below)
lono lore list <game> [--tag <t>] [--subject <id>]  # the authored codex (id, title, tags, subject); no instance needed
lono lore show <game> <id>                     # the full lore entry
lono set <run> <path> --value <v> | --spec '<json>' [--force]  # targeted write (validated by default)
lono rm  <run> <path> [--force]                # remove a node (validated by default)
lono snapshot create <run> [--id <id>] [--label "<l>"]
lono snapshot list <run>
lono snapshot show <run> <snap>
lono snapshot restore <run> <snap> [--into <new-run> | --in-place]
```

Notes:
- `--rel` takes the two endpoints **comma-separated**: `--rel aria,player` (a
  space-separated `--rel aria player` will be misparsed).
- `--seed 0` is treated as unset (a time-based seed is chosen); pass any non-zero
  seed for reproducible dice.
- Mutating commands (`do`, `apply`, `snapshot`) take a per-instance lock, so they
  are safe to run back-to-back.

## Inspect & runtime set/rm

Targeted reads and writes on a running instance — without going through the full
`state` dump or a defined action.

**Read a specific node** (`inspect` is the play-side mirror of `game get`):
```
lono inspect <instance> [path] [--tree] [--depth N]
```
No path → whole state. `--tree` → names-only structural map. Path forms:

| Path | Reads |
|------|-------|
| `entities/<id>/attrs/<a>` | one entity attribute |
| `entities/<id>/inventory/<item>` | item count |
| `entities/<id>/equipped/<slot>` | item type in slot |
| `entities/<id>` | everything for that entity |
| `relationships/<type>/<from>/<to>/attrs/<a>` | one relationship attribute |
| `relationships/<type>/<from>/<to>` | full relationship node |
| `machines/<m>` | current state of a global machine |
| `world/<var>` | a world variable |
| `derived` | all computed derived values |
| `beats` | currently active beats |
| `actions` | available actions |
| `log` | the full narrative journal (`record` entries) |
| `clock` | the current instance clock (tick count) |
| `cooldowns` | active cooldowns and their due ticks |
| `scheduled` | pending scheduled effects and when they fire |
| `discoveredLore` | ids of lore entries the player has revealed so far |

**Write or remove a node** (compiles to validated effect ops by default):
```
lono set <instance> <path> --value <v>      # scalar set
lono set <instance> <path> --spec '<json>'  # structured set (e.g. a full relationship attrs map)
lono rm  <instance> <path>                  # remove (inventory, equipped, relationship, entity)
```

Add `--force` to bypass validation and write raw (use only for debugging or GM
overrides when you know what you're doing).

Path → operation mapping:
- `entities/<id>/attrs/<a>` → `set` op on the entity attribute
- `entities/<id>/inventory/<item>` → `add_item` / `remove_item`
- `entities/<id>/equipped/<slot>` → `equip` / `unequip`
- `relationships/<type>/<from>/<to>/attrs/<a>` → `set_relationship` (attrs patch)
- `machines/<m>` → `set_machine_state`

These are **play-side only**. To change the game's definition (add a character
type, modify a scene, add an item type), use the `creating-a-game` skill and its
`game add`/`define …` commands.

## The output envelope

Every command prints one JSON object to stdout:

```json
{"ok": true,  "command": "do", "data": { ...payload... }}
{"ok": false, "error": {"code": "ACTION_FAILED", "message": "guard not satisfied for action \"escape\"", "details": null}}
```

`ok:false` means nothing changed. Common codes: `NOT_FOUND` (bad id),
`ACTION_FAILED` (illegal/guard-blocked action), `APPLY_FAILED` (bad op),
`BAD_INPUT` (malformed args), `LOCKED` (instance busy), `INSTANCE_EXISTS`
(`play start` would clobber — use a new `--id` or `--force`).

## Reading `data` from `state` / `play start` / `do` / `apply` / `advance`

```json
"data": {
  "state": {
    "world":   {"day": 2, "alarm": false, "loot": 5},
    "machines": {"arc": "inside"},                       // GLOBAL machine scene
    "entities": {
      "player": {"type":"character",
        "attrs": {"name":"Vex","health":80,"location":"vault"},
        "inventory": {"lockpick": 1},
        "equipped": {"torso": "silk_dress"},              // worn items by slot
        "machines": {}},                                  // per-entity attached machines
      "aria": {"type":"character","attrs":{...},"inventory":{},"equipped":{},
        "machines": {}}
    },
    "relationships": [
      {"type":"romance","from":"aria","to":"player",
       "attrs": {"affection": 65, "trust": 20},
       "machines": {"romance_stage": "dating"}}           // per-couple attached machine
    ],
    "deliveredBeats": ["aria_first_smile"]
  },
  "actions": [
    {"machine":"arc","action":"escape","from":"inside","to":"escaped_clean","enabled":true},
    {"machine":"arc","action":"grab_loot","from":"inside","to":"inside",
       "enabled":true,"requiresParams":true,"params":{"bags":{"type":"int","min":1}}},
    {"machine":"romance_stage","action":"start_dating","from":"friends","to":"dating",
       "enabled":false,"reason":"guard not satisfied",
       "host":{"kind":"relationship","from":"aria","to":"player"}}
  ],
  "derived": {
    "global":   {"player_admirers": 1, "top_admirer": "aria"},
    "byEntity": {"aria": {"my_friend_count": 0}}
  },
  "beats": [
    {"id":"aria_first_smile","text":"Aria looks up and smiles at you.","intent":"first warmth"}
  ],
  "endingReached": [
    {"machine":"arc","state":"escaped_clean","description":"You vanish into the night."}
  ],
  "clock": 4,                                            // current in-game tick
  "fired": ["raise_alarm"],                              // triggers that fired this command
  "log": [                                               // recent narrative-journal entries
    {"seq":7,"clock":4,"tags":["aria"],"text":"Aria forgave you and returned the locket."}
  ]
}
```

How to use each field:
- **`actions`** — your menu. Offer the **`enabled:true`** ones. To invoke:
  - global: `do <run> <machine> <action>` (add `--params` if `requiresParams`).
  - attached (has a `host`): add `--rel <from>,<to>` or `--entity <id>`.
  - `enabled:false` carries a `reason` — useful as a hint, but don't offer it.
- **`beats`** — active narrative to weave into this turn's prose. After narrating
  a one-shot beat, deliver it: `apply <run> --ops '[{"op":"mark_beat","beat":"<id>"}]'`.
- **`endingReached`** — non-empty means the story has reached a terminal state;
  narrate the ending (`description`/`intent`) and stop.
- **`derived`** — social-graph summaries for color and pacing ("she's not the
  only one with eyes for you").
- **`state`** — the ground truth for narration: who's where, how they feel
  (`relationships[].attrs`), what's worn (`entities[].equipped`), the scene
  (`machines`).
- **`clock`** — the current in-game tick. Advance time with `advance <run> [n]`
  when fiction passes (a day ends, a deadline approaches) — it fires any due
  scheduled effects and periodic/reactive triggers.
- **`fired`** — names of triggers the engine fired automatically this command.
  These are consequences the engine already applied (state has changed) — read
  them and **narrate the fallout** (the alarm tripped, a deadline elapsed).
- **`log`** — the narrative journal so far (most recent `record` entries). Append
  to it at meaningful moments with `apply <run> --ops '[{"op":"record","text":"…","tags":[…]}]'`;
  on resume, read the full story with `inspect <run> log`.

## Performing actions — examples

```bash
# A plain action:
lono -d ./.lono do run1 arc escape

# An action that needs parameters:
lono -d ./.lono do run1 arc grab_loot --params '{"bags":2}'

# An attached romance-stage action on a specific couple:
lono -d ./.lono do run1 romance_stage start_dating --rel aria,player

# A freeform narrative nudge (player gives Aria a gift -> +affection), via apply:
lono -d ./.lono apply run1 --ops '[{"op":"adjust_relationship","relType":"romance","from":"aria","to":"player","attr":"affection","by":10}]'

# Equip an outfit chosen by the player:
lono -d ./.lono apply run1 --ops '[{"op":"equip","entity":"player","slot":"torso","item":"silk_dress"}]'

# Advance in-game time (e.g. a night passes) — fires deadlines / periodic / reactive triggers:
lono -d ./.lono advance run1 1

# Write a memory to the narrative journal at a meaningful moment:
lono -d ./.lono apply run1 --ops '[{"op":"record","text":"Aria forgave you on the rooftop.","tags":["aria"]}]'
```

Use only documented ops in `apply` (see the authoring reference for the full op
list). The engine validates types/bounds/refs and rejects anything illegal, so a
rejected `apply` means the move wasn't allowed — relay why and continue.

## Worlds, travel & lore

If the game has a map (location entities + `exit` relationships + a `location` ref
on movers — see the authoring reference's **Worlds & maps**), drive it at runtime:

```bash
# Read the scene: where the player is, the exits, who's present.
lono -d ./.lono inspect run1 entities/player/attrs/location          # current room id
lono -d ./.lono inspect run1 derived                                 # if authored: exits_here / whos_here (list)
lono -d ./.lono inspect run1 entities/study                          # the room's authored description + attrs

# Travel: move the player along a connected exit (guarded by via), then pass travel time.
lono -d ./.lono apply run1 --ops '[{"op":"move","entity":"player","to":"hall","attr":"location","via":"exit"}]'
lono -d ./.lono advance run1 1                                       # travel time; fires any arrival triggers
```

A `move` with `via:"exit"` is rejected (`no exit from "study" to "garden"`) when no
exit connects the rooms — relay that as "there's no way through from here." Movement
is usually best authored into a transition's effects (`do`), with `apply` for an
unscripted nudge.

**Lore / codex.** Read the authored world bible for grounding (no instance needed),
and reveal entries as the player learns them:

```bash
lono -d ./.lono lore list mygame --subject locket        # entries about the locket
lono -d ./.lono lore show mygame locket_provenance        # the full entry text
lono -d ./.lono apply run1 --ops '[{"op":"discover","lore":"locket_provenance"}]'   # mark it known
lono -d ./.lono inspect run1 discoveredLore               # what the player has uncovered
```

`discover` is idempotent and errors on an unknown id. Pull facts from `lore
show`/`list` to ground your narration in the authored history; emit `discover` at
the moment the player actually learns something so `discoveredLore` reflects the
playthrough.
