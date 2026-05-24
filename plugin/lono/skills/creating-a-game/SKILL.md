---
name: creating-a-game
description: >-
  Use when the user wants to design, build, author, or create a new story-driven
  game, interactive fiction, visual novel, RPG, or relationship-driven game with
  the lono engine — collaboratively shapes the premise, characters, relationships,
  arc, branching points and endings, then constructs and saves a validated game
  definition. Triggers: "let's make a game", "build a visual novel", "author a
  story game", "create a lono game", "design an interactive story".
allowed-tools: Bash(lono *) Bash(lono-* *)
---

# Creating a lono game

You are collaborating with the user to author a **game definition** — the rules
of a story-driven game — using the `lono` engine. lono enforces the rules: it
validates every type, bound, reference, and guard, and at runtime reports the
current state and the legal actions. Your job here is to turn the user's story
ideas into a definition the engine accepts; the *playing* happens later (see the
`running-a-game` skill).

**Before doing anything else, read `${CLAUDE_SKILL_DIR}/reference.md`** — it is
the complete authoring vocabulary (commands, the definition schema, value types,
guard ops, effect ops, dotted paths, derived values, attached machines, beats,
endings, equipment). Do not invent fields or ops; only use what the reference
documents.

## Two APIs

lono has two cleanly separated APIs — never mix them:

- **Construction** (this skill) — build the game's **definition**: characters,
  relationships, items, events, scenes, branches, endings. Nothing is running.
  Build with `game add|give|rm|list` and `define …`. The definition is
  validated after every change; no instance exists yet.
- **Play** (`running-a-game` skill) — start an instance and take actions:
  `play start`, `do`, `apply`, `inspect`, `snapshot`. Drives a live session.

You build with `game …` / `define …`. You play with `play` / `do` / `apply`.
They never mix.

## The engine

The plugin bundles the engine on your PATH as `lono`. Confirm it runs, then work
in a data dir inside the user's project:

```bash
lono version
```

Pick one data dir for the whole session and pass it every time, e.g.
`--data-dir ./.lono` (or set it once: `export LONO_HOME="$PWD/.lono"`). Every
command prints a JSON envelope: `{"ok":true,"data":{...}}` or
`{"ok":false,"error":{"code":"...","message":"..."}}`. Add `--pretty` while
authoring. **After every change, check `ok` — if false, read the error and fix
it before continuing.**

## New game, or changing an existing one?

Decide this first — they use different commands:

- **New game** → follow "Authoring a new game" below (it starts with `game create`).
- **Changing a game that already exists** — the user says "add…", "change…",
  "remove…", "tweak…", "rename…", "fix…" to a game that's already built or saved →
  go to **"Modifying an existing game"**. Do **not** run `game create` on it and do
  **not** re-`game import` the whole thing to make a small change — both replace the
  *entire* definition and discard everything else. (`game create` on an existing id
  now errors for this reason; use targeted edits.)

## Modifying an existing game

1. **Find it.** If it only exists as a saved file, load it once:
   `lono --data-dir ./.lono game import --spec-file <file>.lono.json`. See what's
   loaded with `lono --data-dir ./.lono game list`.
2. **Introspect — read before you change.** Navigate to the exact node you need:
   `lono --data-dir ./.lono game get <id> <path>` (e.g. `machines/arc/transitions/begin`,
   `entities/aria`), or get a names-only structural overview with
   `lono --data-dir ./.lono game get <id> --tree`. For the full raw dump:
   `lono --data-dir ./.lono game show <id> --pretty`.
3. **Change only that piece** with a targeted `define … set`/`rm`. Each edits one
   named element in place and leaves the rest untouched:

   | To change… | Command | Effect |
   |---|---|---|
   | a world var | `define var set <id> <name> --spec …` / `define var rm <id> <name>` | replaces/removes just that var |
   | an entity / item / rel type | `define entity-type\|item\|relationship-type set <id> <name> --spec …` / `rm` | replaces/removes just that type (`item` and `relationship-type` are aliases) |
   | a derived value / a beat | `define derived\|event set <id> <name> --spec …` / `rm` | replaces/removes just that one (`event` is an alias for `beat`) |
   | **one transition** | `define branch set <id> <machine> --spec '{"id":"…", …}'` | **upserts that transition by its `id`**; sibling transitions untouched (`branch` is alias of `transition`) |
   | remove one transition | `define branch rm <id> <machine> <transitionId>` | removes just that transition |
   | one state's stateMeta | `define scene set <id> <machine> <state> --spec '<StateMeta>'` | sets only that state's metadata; leaves others and transitions untouched |
   | a machine's states / initial / stateMeta | `define machine set <id> <machine> --spec …` | ⚠ **REPLACES THE WHOLE MACHINE**, dropping its transitions — re-include them in the spec, or (better) edit transitions one-by-one with the rows above |

   To edit `setup`, the game `description`, or `intent` (no granular command),
   `game show` the definition, change just those keys in the JSON, and `game import`
   the modified whole — that is the one case where a full re-import is correct.
   If the user only wants to nudge a value in a *running session* (not the rules),
   that's a play-time change — use `apply` in the `running-a-game` skill instead.
4. **Validate & re-save.** `game validate <id>`, then re-export to the user's file:
   `game export <id> -o <file>.lono.json`.

The rule: read the current value, send only the delta, validate, re-export — never
rewrite the whole game to change one thing.

## Authoring a new game

Work through the design **collaboratively and one area at a time**. Ask the user
about each area, propose concrete options, capture their decisions, then encode
them. Don't dump the whole schema at them — translate their story into the model.

Create the empty game first:

```bash
lono --data-dir ./.lono game create <id> --name "<Title>"
```

Then build it up, validating as you go (`lono --data-dir ./.lono game validate <id>`).
Use `define <kind> set <game> <name> --spec '<json>'` for each piece. The areas,
in a good order:

1. **Premise & tone.** What's the story, setting, and mood? Capture as the
   game's `description`/`intent` (set via `game import` of a small patch or note
   it for stateMeta/beats later).
2. **World state.** Global facts that change: time/day, money, alarm raised,
   chapter flags. → `define var`.
3. **Characters & the player.** Who exists? Their attributes (name, health,
   mood, traits). Define the template with `define entity-type`, then add the
   concrete starting cast using the first-class construction commands:
   `game add character <game> <id> --type <type> [--attrs '<json>']`,
   `game give <game> <character> --item <item> [--count N] [--equip <slot>]`,
   and review with `game list characters <game>`. These populate the
   definition's first-class `entities` section (seeded automatically when a
   game starts). Raw `setup` ops remain for advanced scripted seeding and run
   after the declarative cast.
4. **Relationships & social axes.** How do characters feel about each other?
   For a relationship-driven game, one `romance` edge carrying several numeric axes
   (affection, trust, tension, attraction) is usually best; directed edges let
   feelings be asymmetric. → `define relationship-type` (alias: `rel-type`).
   Add concrete starting links with `game add relationship <game> <type> <from>
   <to> [--attrs '<json>']` and review with `game list relationships <game>`.
   These populate the definition's first-class `relationships` section.
5. **Items & equipment.** Inventory items, and worn clothing/accessories/gear
   with slots. → `define item` (alias of `item-type`) + `slots` on the
   character entity-type. Use `game give` to hand items to the starting cast.

   **Worlds & lore (if the story has places to move through or a world to
   ground it).** Model the map from existing primitives: each **place** is a
   `location` entity, **connections** are an **`exit` relationship type** between
   locations (directed, with a `direction` attr), and a mover's position is a
   `location` **ref** attribute. Give rooms/objects/people their own authored
   `--description` on `game add` so a specific place reads distinctly. Encode
   world history/backstory/provenance as **`lore`** entries (`define lore set`,
   with `subject`/`tags`/`when`) — the queryable world bible, separate from beats
   and the journal. At play time, "exits from here"/"who's here" are `derived`
   queries using `{"$path":…}` + the `list` reducer, and travel is the `move` op
   (often guarded `via:"exit"`) paired with `advance` for travel time. See the
   reference's **Worlds & maps** and **Lore / codex**.
6. **The narrative arc.** The spine of the story as a state machine: scenes/
   phases as states, and the actions that move between them as transitions with
   guards (when they're allowed) and effects (what they change). Add
   `description`/`intent` to states and transitions. → `define machine` +
   `define branch` (alias of `transition`) + `define scene` (sets one state's
   `stateMeta` without replacing the whole machine).
7. **Per-couple / per-character arcs.** Romance stages or NPC moods as
   **attached machines** (a template instantiated per relationship/entity). →
   `define machine` with `attach`.
8. **Branching points & endings.** This is the payoff. Declare the endings as
   terminal states (`stateMeta` with `terminal:true`, a `description`, and an
   `intent` describing in plain language when it happens), and the transitions
   into them carry the structured guards that enforce those conditions. Aim for
   the 2–4 distinct endings the user wants.
9. **Derived social queries.** "Does anyone adore the player?", "how many
   friends?" → `define derived` (reusable; read in guards and beats).
10. **Story beats.** Authored prose the engine surfaces at the right moment
    (gated by a state and/or a guard). → `define event` (alias of `beat`).
11. **Reactive rules & time.** Encode automatic consequences and timers as
    **triggers** (rules that fire by themselves when a condition arises) plus
    `schedule`d/`cooldown` effects and the `advance` clock — so the *engine*
    enforces "when the alarm sounds the guard turns hostile" or "the deadline
    hits in 3 turns", not the narrator. Quantitative outcomes (damage, costs,
    skill checks) belong in `compute`/`if`/`$path`/`roll.<store>` so the engine
    does the math rather than the model. Use `set` collections for things you
    accumulate — clues found, party members, topics discussed — and the `record`
    journal op to log consequential moments so the story persists across
    sessions. → `define trigger`; `set`-typed vars/attrs via `define var`/
    `define entity-type`.
12. **Setup.** The starting cast is seeded from the first-class `entities` and
    `relationships` sections (populated by `game add`/`give`). For advanced
    scripted seeding — rolls, conditional ops, anything not expressible
    declaratively — add a `setup` effect-op list; it runs after the cast is
    seeded. Set via `game import` or include in the definition you build.

Throughout: prefer encoding *conditions* as structured guards (the engine
enforces them) while writing the *story* as `description`/`intent`/beat text
(the engine just serves it). Use natural language freely in those text fields —
that is how the player-facing narration gets authored.

## Test as you build

After the arc and a couple of endings exist, dry-run it so you and the user can
feel the flow:

```bash
lono --data-dir ./.lono play start <id> --id _draft --seed 1 --pretty
lono --data-dir ./.lono state _draft --pretty       # current scene + legal actions + beats
lono --data-dir ./.lono do _draft <machine> <action>
```

Walk a few actions toward an ending. Then delete the draft instance's data or
just start fresh. Fix anything that doesn't reach the intended endings.

## Save it

When the user is happy, validate and export the definition to a file they keep:

```bash
lono --data-dir ./.lono game validate <id>
lono --data-dir ./.lono game export <id> -o <id>.lono.json
```

`<id>.lono.json` is the portable, self-contained game. To resume authoring later,
`lono game import --spec-file <id>.lono.json`. To *play* it, hand it to the
`running-a-game` skill.

## Principles

- **The engine is the rule-keeper.** Never put game logic in prose; put it in
  guards/effects. If you catch yourself "remembering" a rule, encode it.
- **Validate relentlessly.** Run `game validate` after each area; fix `ok:false`
  immediately. A definition that doesn't validate can't be played.
- **Iterate with the user.** Propose, encode, show the result, refine. Keep the
  story theirs.
- **YAGNI.** Only model what the story needs. A small, valid, playable game beats
  a sprawling invalid one.
