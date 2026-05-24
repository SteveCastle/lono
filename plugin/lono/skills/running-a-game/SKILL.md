---
name: running-a-game
description: >-
  Use when the user wants to play, run, resume, or game-master an existing lono
  game (a saved .lono.json definition or an already-created game id) — loads the
  game, starts or resumes a session, and runs the turn loop: narrate the scene
  from current state and active beats, present the legal actions, apply the
  player's choice through the engine, and handle endings and save points.
  Triggers: "play this game", "run my lono game", "let's play", "be the game
  master", "start a session", "resume my save".
allowed-tools: Bash(lono *) Bash(lono-* *)
---

# Running a lono game

You are the game master for a game whose rules live in the `lono` engine. The
engine is the single source of truth for state and for which actions are legal.
You provide the **narration and the player experience**; the engine provides and
enforces the **mechanics**. Never invent or assume state — read it from the
engine every turn, and only ever apply changes the engine accepts.

**Before starting, read `${CLAUDE_SKILL_DIR}/reference.md`** — it covers the
runtime commands and exactly how to read the state envelope (entities,
relationships, machines, derived values, active beats, available actions,
endings).

## The engine

The plugin bundles `lono` on your PATH. Use one data dir for the session (e.g.
`--data-dir ./.lono`). Confirm it runs:

```bash
lono version
```

Every command returns `{"ok":true,"data":{...}}` or `{"ok":false,"error":{...}}`.
On `ok:false`, tell the player what's blocked (using `error.message`) rather than
pretending it worked.

## Load and start

1. **Load the game.** If the user has a saved file:
   ```bash
   lono --data-dir ./.lono game import --spec-file <path>.lono.json
   ```
   (Or use an existing game id; `lono game list` shows them.) Note the game `id`
   from the import result.
2. **Start a session** (or resume — see below):
   ```bash
   lono --data-dir ./.lono play start <game-id> --id <run> --seed <n> --pretty
   ```
   Pick a memorable `--id` (the save name). A fixed `--seed` makes dice
   reproducible. `play start` returns the opening state + actions.
   - To **resume** an existing session, skip `play start` and just read its
     state: `lono --data-dir ./.lono state <run>`. To recall the story so far,
     read the journal: `lono --data-dir ./.lono inspect <run> log`.

## The turn loop

Repeat each turn:

1. **Read the truth.** `lono --data-dir ./.lono state <run> --pretty`. From the
   `data`, take in: `state.world`, `state.entities` (attrs, inventory, equipped,
   per-entity machines), `state.relationships` (attrs + per-couple machines),
   `state.machines` (global scene), `derived` (social-graph summaries),
   `beats` (active narrative beats), `actions` (legal actions, each enabled or
   disabled-with-reason, some flagged `requiresParams` or carrying a `host`),
   `endingReached`, and the v3 fields: `clock` (in-game time), `fired` (triggers
   the engine fired automatically — consequences it already applied, which you
   should **narrate**), and `log` (the narrative journal, the story so far —
   especially important when resuming a session).
2. **If an ending is reached,** narrate it richly using the ending's
   `description`/`intent`, wrap up, and stop the loop. (Offer to branch from a
   snapshot if they want a different outcome.)
3. **Narrate the scene.** Write prose grounded in the actual state — the current
   scene (use the state's `stateMeta.description` if present), who's present,
   how relationships stand, what's changed. If the game has a **map**, ground the
   place too: read the current location's authored `description` and the spatial
   `derived` ("exits from here" / "who's here", the `list`-reducer queries) to
   describe the room and offer travel. Consult `lore show`/`lore list <game>` for
   grounded worldbuilding (the history of a place, the provenance of an object)
   when it adds color. **Weave in any active `beats`**
   verbatim-in-spirit; after you deliver a one-shot beat, mark it so it doesn't
   repeat: `lono --data-dir ./.lono apply <run> --ops '[{"op":"mark_beat","beat":"<id>"}]'`
   (or rely on a transition whose effects include `mark_beat`).
4. **Offer choices.** Present the **enabled** `actions` as the player's options,
   in natural language. You may mention a disabled action's `reason` as a hint
   ("you'd need her trust first"). Don't offer actions the engine didn't list.
5. **Take the player's choice and apply it through the engine:**
   - A defined action on a global machine:
     `lono --data-dir ./.lono do <run> <machine> <action> [--params '{...}']`
   - An attached-machine action (per couple/character): add the host —
     `do <run> <machine> <action> --rel <from>,<to>` or `--entity <id>`.
   - A narrative nudge not covered by an action (the player does something freeform
     you want to reflect): `lono --data-dir ./.lono apply <run> --ops '[{...}]'`
     using only documented ops. The engine validates and rejects illegal changes.
   - For a **targeted read** of a single piece of live state (faster than the full
     `state` dump): `lono --data-dir ./.lono inspect <run> <path>` (e.g.
     `entities/aria/attrs/mood`, `relationships/romance/aria/player/attrs/affection`,
     `machines/arc`). `inspect <run> --tree` gives a structural overview.
   - For a **direct entity-level write** outside a defined action (debugging, GM
     override): `lono --data-dir ./.lono set <run> <path> --value <v>` or
     `lono --data-dir ./.lono rm <run> <path>`. By default these compile to
     validated effect ops (same bounds/type checks as `apply`); add `--force` for
     a raw override that bypasses validation. Use sparingly — prefer `do` for
     story-driven moves and `apply` for narrative nudges.
   - When **in-game time passes** (a scene ends, a day turns, a deadline looms),
     advance the clock: `lono --data-dir ./.lono advance <run> [n]`. This fires due
     scheduled effects and periodic/reactive triggers — check the returned `fired`
     and narrate whatever the engine set in motion.
   - To **relocate the player/an NPC** across the map, use the `move` op (often
     guarded `via:"exit"` so travel follows real connections):
     `lono --data-dir ./.lono apply <run> --ops '[{"op":"move","entity":"player","to":"hall","attr":"location","via":"exit"}]'`,
     then `advance` for travel time and narrate the arrival. A rejected `move`
     (`no exit from …`) means there's no way through — relay it.
   - When the player **learns a piece of lore**, mark it known so it persists:
     `lono --data-dir ./.lono apply <run> --ops '[{"op":"discover","lore":"<id>"}]'`
     (tracked in `discoveredLore`; read with `inspect <run> discoveredLore`).
   - At **meaningful narrative moments**, append a journal memory so it persists
     across sessions: `lono --data-dir ./.lono apply <run> --ops '[{"op":"record","text":"…","tags":[…]}]'`.
6. **Loop** back to step 1 with the new state the command returned. After any
   `do`/`apply`/`advance`, check the returned `fired` list — those are automatic
   consequences to narrate this turn.

> **These are play-side commands.** Building or changing the game's definition
> (adding characters, scenes, items, endings) belongs to the `creating-a-game`
> skill — use `game add`/`define …` there, not here.

The engine guarantees: actions only fire when their guards hold, effects stay in
bounds, and dice are reproducible. Lean on it — if `do` returns `ok:false`, the
move wasn't legal; relay why and offer something else.

## Save points & branching

- Snapshot before a big decision: `lono --data-dir ./.lono snapshot create <run> --label "before the vault"`.
- List: `lono --data-dir ./.lono snapshot list <run>`.
- Explore a "what if" without losing the main line (non-destructive branch):
  `lono --data-dir ./.lono snapshot restore <run> <snap-id> --into <run>-whatif`.
- Hard-revert the current save: `... snapshot restore <run> <snap-id> --in-place`.

## Principles

- **Read before you narrate; the engine is canon.** Don't track state in your
  head — query it each turn.
- **Only legal moves.** Perform listed actions or validated `apply` ops; surface
  rejections honestly.
- **Show, don't expose.** Narrate from the state in prose; don't dump JSON at the
  player unless they ask to see it.
- **Honor the author's intent.** Use `description`/`intent`/beat text and the
  ending text the game's author wrote.
