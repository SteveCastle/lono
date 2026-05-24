# lono

**lono** is a CLI state engine for story-driven games — interactive fiction,
RPGs, visual novels, branching narratives — built to be driven by an LLM. The model
provides the *narrative*; lono owns the *rules and the state*. Every change is
validated against the game's definition (types, bounds, references, action
guards), so the model can't drift the game into an inconsistent state.

Think of it as the **rules engine + save file** behind a text game, with a
clean, JSON-in/JSON-out command surface an LLM (or any tool-calling client) can
operate.

---

## Contents

- [Core idea](#core-idea)
- [How it works](#how-it-works)
- [Install](#install)
- [Quickstart: build and play a game](#quickstart-build-and-play-a-game)
- [The two APIs: Construction vs Play](#the-two-apis-construction-vs-play)
- [The game model](#the-game-model)
- [The rule language: guards, effects, paths](#the-rule-language-guards-effects-paths)
- [Playing a game: the turn loop](#playing-a-game-the-turn-loop)
- [Command reference](#command-reference)
- [Output, data dir, and errors](#output-data-dir-and-errors)
- [Claude Code plugin](#claude-code-plugin)
- [Project layout & further reading](#project-layout--further-reading)

---

## Core idea

There are two things in lono, and keeping them straight is the whole mental model:

- A **definition** — the *rules* of a game: its world variables, the kinds of
  characters and items, relationships, the storyline state machines, narrative
  beats, endings, and the starting cast. A definition is a single JSON document
  you author and save (`<game>.lono.json`). **No game is "running" here.**
- An **instance** — a *running playthrough* seeded from a definition: the live
  world values, the concrete characters and how they feel about each other, where
  each storyline currently sits, what's been narrated, a reproducible RNG. You
  start one, read its state, and take actions.

The engine **enforces** the definition over the instance: an action only fires
if its guard holds, a value can't exceed its bounds, a relationship can't point
at a character who doesn't exist, dice are reproducible from a seed.

```mermaid
flowchart LR
    subgraph Construct["Construction API (author)"]
      A[LLM + author] -->|game add / define / get| DEF[(definition.json<br/>rules + cast)]
    end
    subgraph Play["Play API (run)"]
      DEF -->|play start| INST[(instance state<br/>live playthrough)]
      P[LLM game-master] -->|state / inspect| INST
      P -->|do / apply / set| INST
    end
    ENG{{lono engine<br/>validates every change}}
    DEF -. enforced by .- ENG
    INST -. enforced by .- ENG
```

---

## How it works

lono is a small, pure Go engine behind a stateless CLI. Each command reads the
relevant JSON from a data directory, performs one validated operation, writes it
back, and prints a JSON envelope. There is no daemon — the binary is the API.

```mermaid
flowchart TD
    CLI["lono CLI (cobra)"] --> ENGINE["engine package (pure, no I/O)<br/>model · guards · effects · machines · validate · rng"]
    CLI --> STORE["store (filesystem JSON + locking)"]
    STORE --> FS["data-dir/<br/>games/ID/definition.json<br/>instances/ID/state.json<br/>instances/ID/snapshots/*.json"]
    ENGINE -. used by .- STORE
```

The `engine` package does no I/O and is fully unit-tested; the CLI is one
frontend over it (an MCP server or function-tool adapter could be another).

---

## Install

Requires Go 1.23+.

```bash
git clone https://github.com/callsignmedia/lono
cd lono
go build -o lono ./cmd/lono      # produces ./lono (or lono.exe on Windows)
```

Or use it inside Claude Code via the bundled plugin — see
[Claude Code plugin](#claude-code-plugin) (no Go toolchain needed there).

Every example below assumes `lono` is on your PATH and uses `--data-dir ./.lono`
to keep all game data in one folder.

---

## Quickstart: build and play a game

Build a tiny gallery vignette **entirely through the Construction API**, then
play it.

```bash
D="--data-dir ./.lono"

# 1) Create the game and its rules/templates
lono $D game create gallery --name "A Night at the Gallery"
lono $D define entity-type set gallery character \
  --spec '{"attributes":{"name":{"type":"string"},"mood":{"type":"int","default":50,"min":0,"max":100}}}'
lono $D define relationship-type set gallery romance \
  --spec '{"from":"character","to":"character","directed":true,
           "attributes":{"affection":{"type":"int","default":0,"min":-100,"max":100}}}'
lono $D define item set gallery champagne --spec '{"category":"drink"}'

# 2) The storyline: a state machine with a guarded "happy" ending
lono $D define machine set gallery arc \
  --spec '{"initial":"mingling","states":["mingling","together","alone"]}'
lono $D define scene set gallery arc together \
  --spec '{"terminal":true,"ending":true,"description":"You leave the gallery together."}'
lono $D define scene set gallery arc alone \
  --spec '{"terminal":true,"ending":true,"description":"You leave alone."}'
lono $D define branch set gallery arc \
  --spec '{"id":"leave_together","from":"mingling","to":"together",
           "guard":{"target":"rel.romance.aria.player.affection","op":"gte","value":50}}'
lono $D define branch set gallery arc \
  --spec '{"id":"leave_alone","from":"mingling","to":"alone"}'

# 3) The cast (first-class characters + a starting relationship)
lono $D game add character gallery player --type character --attrs '{"name":"You"}'
lono $D game add character gallery aria   --type character --attrs '{"name":"Aria"}'
lono $D game add relationship gallery romance aria player --attrs '{"affection":10}'

# 4) A narrative beat the engine surfaces at the right moment
lono $D define event set gallery aria_smiles \
  --spec '{"text":"Aria catches your eye across the room and smiles.",
           "guard":{"target":"rel.romance.aria.player.affection","op":"gte","value":40}}'

# 5) Validate and save the portable definition
lono $D game validate gallery
lono $D game export gallery -o gallery.lono.json

# ---- play it ----
lono $D play start gallery --id run1 --seed 42 --pretty   # cast is seeded automatically
lono $D state run1                                         # scene + characters + actions + beats
lono $D apply run1 --ops '[{"op":"adjust_relationship","relType":"romance","from":"aria","to":"player","attr":"affection","by":45}]'
lono $D state run1                                         # the beat is now active; leave_together is enabled
lono $D do run1 arc leave_together                         # -> reaches the "together" ending
```

The model would narrate around each step; lono keeps the bookkeeping honest.

---

## The two APIs: Construction vs Play

They never mix. You **build** with one set of commands and **play** with another.

```mermaid
flowchart LR
    subgraph C["Construction API — edits a definition"]
      direction TB
      c1["game create / import / export"]
      c2["game add character|relationship · give"]
      c3["define var/entity-type/item/rel-type/<br/>machine/branch/scene/derived/event"]
      c4["game get / --tree  (introspect)"]
      c5["game validate"]
    end
    subgraph P["Play API — drives an instance"]
      direction TB
      p1["play start / list"]
      p2["state · actions · inspect"]
      p3["do  (guarded action)"]
      p4["apply (validated ops) · set / rm"]
      p5["snapshot create/list/restore"]
    end
    C -->|"export to file, then import"| P
```

- **Construction** mutates the *definition* only; nothing is running. Every edit
  is re-validated against the whole definition and rejected atomically if invalid.
- **Play** drives a running *instance*; it never changes the rules.

---

## The game model

A definition is a tree. Here's everything it can contain:

```mermaid
flowchart TD
    DEF["definition (a game)"]
    DEF --> W["world<br/>typed global variables"]
    DEF --> ET["entityTypes<br/>character/thing templates:<br/>attributes + equipment slots"]
    DEF --> IT["itemTypes<br/>category, equippable, attributes"]
    DEF --> RT["relationshipTypes<br/>directed links + attributes"]
    DEF --> M["machines<br/>storylines: states, transitions,<br/>endings; global or attached"]
    DEF --> DV["derived<br/>aggregate queries over the graph"]
    DEF --> B["beats<br/>authored narrative events"]
    DEF --> CAST["entities + relationships<br/>the starting cast"]
```

| Resource | What it is |
|---|---|
| **world var** | a typed global value: `int`/`float` (with min/max), `bool`, `string`, `enum`, `ref` |
| **entity type** | a template (e.g. `character`) with typed attributes and equipment **slots** |
| **item type** | a thing that can be held or worn: `category`, `equippable`, static attributes |
| **relationship type** | a directed/undirected link kind between entity types, with its own attributes (e.g. `romance` carrying `affection`, `trust`) |
| **machine** | a storyline as a state machine. *Global* machines track the overall arc; **attached** machines instantiate **per couple or per character** (e.g. a romance-stage arc for each pair) and read their host via `this.*` |
| **scene / ending** | a state of a machine; mark it `terminal`+`ending` and the engine reports when it's reached |
| **branch** | a transition/action between states, gated by a **guard** and applying **effects** |
| **derived value** | a reusable aggregate over the social graph — `count`/`any`/`sum`/`min`/`max`/`argmax` — e.g. "how many characters adore the player?" |
| **beat** | authored prose the engine surfaces as *active* when its state/guard conditions hold |
| **cast** | the concrete starting **entities** (characters/items in the world) and their starting **relationships** |

A sample storyline machine:

```mermaid
stateDiagram-v2
    [*] --> mingling
    mingling --> together: leave_together [affection >= 50]
    mingling --> alone: leave_alone
    together --> [*]
    alone --> [*]
```

---

## The rule language: guards, effects, paths

Conditions and changes are **structured JSON** (no embedded scripting), so they're
fully validatable and easy for a model to emit.

**Paths** address any value in the game:

```
world.<var>                         entity.<id>.<attr>
inventory.<id>.<item>               equipped.<id>.<slot>      worn.<id>.<slot>.<attr>
rel.<type>.<from>.<to>.<attr>       machine.<name>.state
derived.<name>                      entity.<id>.derived.<name>
itemtype.<id>.<attr>                param.<name>
this.<attr> / this.from / this.to   (inside an attached-machine transition)
```

**Guards** are condition trees — leaves `{target, op, value}` with
`eq ne gt gte lt lte in exists`, combined with `and`/`or`/`not`:

```json
{"and":[
  {"target":"entity.player.health","op":"gt","value":0},
  {"target":"rel.romance.aria.player.affection","op":"gte","value":50}
]}
```

**Effects** are an ordered op list applied atomically:

```
set · inc · dec · mul                         add_item · remove_item
create_entity · destroy_entity                equip · unequip
set_relationship · adjust_relationship · remove_relationship
set_machine_state · set_attached_state        roll (seeded dice)   mark_beat
```

The engine validates every effect (types, bounds, references) before committing,
and rolls draw from a per-instance seed so playthroughs are reproducible.

---

## Playing a game: the turn loop

```mermaid
sequenceDiagram
    participant U as Player
    participant M as LLM game-master
    participant L as lono
    loop each turn
        M->>L: state run
        L-->>M: world, characters, relationships, scene,<br/>active beats, legal actions, endings
        M->>U: narrate the scene + offer the legal choices
        U->>M: a choice
        M->>L: do run machine action   (or apply / set)
        L-->>M: ok + new state  (or ok:false + why it is illegal)
    end
    Note over M,L: on an ending, narrate it and stop
```

The contract for the driver: **read state every turn, only perform listed actions
or validated ops, never invent state.** lono is the source of truth.

---

## Command reference

Run any command with `--data-dir <dir>` (or set `$LONO_HOME`); add `--pretty` to
indent JSON.

### Game lifecycle
```
lono game create <id> --name "<title>" [--force]
lono game list | show <id> | delete <id> | validate <id>
lono game export <id> -o <file>           # save the portable definition
lono game import --spec-file <file>       # load a whole definition (validated)
lono game get <id> [path] [--tree] [--depth N]   # navigate/inspect the definition
```

### Construction — cast
```
lono game add character <id> <char> --type <type> [--attrs '<json>']
lono game add relationship <id> <type> <from> <to> [--attrs '<json>']
lono game give <id> <char> --item <item> [--count N] [--equip <slot>]
lono game rm character <id> <char> | rm relationship <id> <type> <from> <to>
lono game list characters <id> | list relationships <id>
```

### Construction — types, world, story
```
lono define var|entity-type|item|relationship-type|derived  set|rm <id> <name> --spec '<json>'
lono define machine  set|rm <id> <name>    --spec '<json>'
lono define branch   set|rm <id> <machine> --spec '<transition json>'   # transition, upsert by id
lono define scene    set|rm <id> <machine> <state> --spec '<stateMeta json>'
lono define event    set|rm <id> <name>    --spec '<beat json>'
```
(`item`=item-type, `relationship-type`=rel-type, `branch`=transition, `event`=beat — friendly aliases.)

### Play
```
lono play start <game> --id <run> [--seed N]
lono play list
lono state <run>                          # full state + actions + beats + endings
lono actions <run>
lono inspect <run> [path] [--tree]        # targeted read of live state
lono do <run> <machine> <action> [--params '<json>'] [--rel <from>,<to> | --entity <id>]
lono apply <run> --ops '<json array of effects>'
lono set <run> <path> --value <v> | --spec '<json>' [--force]   # entity-level write (validated; --force = raw)
lono rm  <run> <path> [--force]
lono snapshot create <run> [--label "<l>"] | list <run> | show <run> <snap>
lono snapshot restore <run> <snap> [--into <new-run> | --in-place]
```

---

## Output, data dir, and errors

Every command prints one JSON envelope to stdout and exits non-zero on failure:

```json
{ "ok": true,  "command": "do", "data": { "state": {}, "actions": [] } }
{ "ok": false, "error": { "code": "ACTION_FAILED", "message": "guard not satisfied", "details": null } }
```

The data directory is resolved as `--data-dir` → `$LONO_HOME` → `./.lono`. Layout:

```
<data-dir>/
  games/<id>/definition.json
  instances/<id>/state.json
  instances/<id>/snapshots/<snap>.json
```

Common error codes: `NOT_FOUND`, `INVALID_DEFINITION` (with a `details` list),
`ACTION_FAILED`, `APPLY_FAILED`, `GUARD_FAILED`, `NO_SUCH_PATH`,
`PATH_NOT_WRITABLE`, `GAME_EXISTS`, `INSTANCE_EXISTS`, `LOCKED`, `BAD_INPUT`.

---

## Claude Code plugin

lono ships as a Claude Code plugin (`plugin/`) so you can build and play games
conversationally, with the engine bundled (prebuilt binaries for macOS/Linux/
Windows — no Go needed):

```
/plugin marketplace add /path/to/lono/plugin
/plugin install lono@lono-marketplace
```

It adds two skills:

- **`creating-a-game`** — collaboratively design a game and build a validated,
  saveable definition (the Construction API).
- **`running-a-game`** — load a saved game and game-master the turn loop (the
  Play API).

See [`plugin/lono/README.md`](plugin/lono/README.md) for details.

---

## Project layout & further reading

```
cmd/lono/            CLI entrypoint
internal/engine/     pure engine: model, guards, effects, machines, derived,
                     narrative (beats/endings), equipment, navigation, validation, rng
internal/store/      filesystem persistence + per-instance locking
internal/cli/        cobra commands (the JSON-envelope frontend)
testdata/            golden example games used in the test suite
plugin/              the Claude Code plugin (skills + bundled binaries)
```

The full command/vocabulary reference (data model, ops, guards, paths) lives in
the bundled skills' `reference.md` files under
`plugin/lono/skills/*/reference.md`.
