# Example games

Five ready-to-run lono definitions, each a single `*.lono.json` you can import and
play. They progress from a tiny tour of *every* system to focused showcases of one
area each. Use them to learn the vocabulary, to copy patterns into your own game,
or as fixtures while exploring the CLI.

| File | Theme | Focus | Endings |
|------|-------|-------|---------|
| [`quickstart.lono.json`](quickstart.lono.json) | midnight manor escape | **everything, briefly** — a short tour touching every system | escaped / caught |
| [`gallery-romance.lono.json`](gallery-romance.lono.json) | art-gallery soirée | social graph: multi-axis relationships, attached machines, derived queries, beats | together / alone / love-triangle |
| [`vault-heist.lono.json`](vault-heist.lono.json) | bank heist | items, equipment, dice skill-checks | escaped clean / escaped hot / caught |
| [`last-watch.lono.json`](last-watch.lono.json) | siege survival | reactive systems & time: triggers, clock, schedule, cooldowns, sets, journal | relieved / fallen |
| [`hollow-manor.lono.json`](hollow-manor.lono.json) | murder mystery | worlds & maps: locations, travel, spatial queries, lore/codex | solved / wrong accusation |

## Running an example

All commands take a `--data-dir` (shown as `-d ./.lono`); use the same one for a
whole session. Build the binary first (`go build -o lono ./cmd/lono`) or use the
one bundled with the plugin.

```bash
# 1. Load the definition (validated on import)
lono -d ./.lono game import --spec-file examples/quickstart.lono.json

# 2. Start a session (use a non-zero --seed for reproducible dice)
lono -d ./.lono play start quickstart --id run1 --seed 42

# 3. Read the scene: state + available actions + active beats + endings reached
lono -d ./.lono state run1 --pretty

# 4. Take actions until an ending is reached
lono -d ./.lono do run1 <machine> <action>          # global action
lono -d ./.lono do run1 <machine> <action> --rel a,player   # attached (relationship) action
lono -d ./.lono advance run1 1                        # pass a tick (fires schedule/periodic/reactive)
```

`game validate <id>` checks any imported definition. See the plugin's
`running-a-game` skill (or `plugin/lono/skills/running-a-game/reference.md`) for
the full command and output reference.

---

## The games

### quickstart — a short tour of every system
A burglar slips out of a manor before the alarm trips. Deliberately small, but it
exercises **world vars, a set attribute, an entity type with an equip slot, an
equippable item, a cast (player + guard) with descriptions, a relationship, an
attached machine, a global arc with endings, guards (equipped/cooldown/contains/
clock), a `roll` + `compute` + `if/then/else` skill-check, `add_to` a set,
`discover` lore, `mark_beat`, `record`, `cooldown`, `schedule`, `move via` an exit,
`equip`, beats (one-shot + repeatable), a reactive trigger, a periodic trigger,
`advance`, a derived spatial query, and a lore entry.**

```bash
lono -d ./.lono game import --spec-file examples/quickstart.lono.json
# Escape (seed 42 → sneak roll 14 ≥ DC 8):
lono -d ./.lono play start quickstart --id win --seed 42
#   …take the sneak action, then the exit action → "escaped"
# Get caught (trip the alarm, let the scheduled capture fire):
lono -d ./.lono play start quickstart --id lose --seed 99
lono -d ./.lono set lose world/alarm --value true
lono -d ./.lono advance lose 2     # on_alarm trigger → scheduled capture → "caught"
```

### gallery-romance — relationships, attached machines, derived, beats
A soirée with three potential partners. Shows the **social graph in depth**: a
`romance` relationship carrying three axes (`affection`, `tension`, `trust`); an
**attached** `romance_stage` machine per couple (`strangers → acquaintances →
flirting → smitten`) whose transitions are guarded on `this.affection` /
`this.trust` and on an equipped outfit; **derived** queries (`admirer_count`,
`top_admirer` via `argmax`, `tension_sources` via `list`); guarded **beats**; and
three **endings** gated on the relationship state.

```bash
lono -d ./.lono game import --spec-file examples/gallery-romance.lono.json
lono -d ./.lono play start gallery-romance --id r1 --seed 42
# Warm up to a partner with the attached romance machine, then leave together:
lono -d ./.lono do r1 romance_stage chat --rel celeste,player
lono -d ./.lono do r1 romance_stage flirt --rel celeste,player
#   …raise affection through the stages (charm needs the velvet jacket equipped)…
lono -d ./.lono do r1 arc exit_together        # → "left_together"
```

### vault-heist — inventory, equipment, dice skill-checks
Crack a vault and get out. Shows **items consumed by actions** (lockpicks, a
single-use security card), **equipment** that gates or eases actions (a disguise
to enter, a crowbar to force the safe), and a **dice skill-check**: `crack_safe`
rolls `1d20`, and on success a `compute` adds the loot while on failure it trips
`world.alarm`. Guards gate every step; three **endings** branch on loot + alarm.

```bash
lono -d ./.lono game import --spec-file examples/vault-heist.lono.json
# Clean escape (seed 3 → safe-crack roll succeeds):
lono -d ./.lono play start vault-heist --id clean --seed 3
#   enter_lobby → take_stairs_to_vault → bypass_vault_door → grab_petty_cash
#   → crack_safe (success) → escape_clean   → "escaped_clean"
# Caught (seed 300 → crack fails, alarm trips, scheduled capture fires):
lono -d ./.lono play start vault-heist --id bust --seed 300
#   …crack_safe (fail) → advance 3 → "caught"
```

### last-watch — triggers, clock, schedule, cooldowns, sets, journal
Hold an outpost until relief arrives. Shows the **reactive layer end to end**: a
**periodic** trigger (`every:1`) that drains supplies/morale each night and writes
the **journal**; **reactive** triggers (starvation when supplies hit 0, a breach
check, relief arrival); a **scheduled** deadline (relief queued for tick 3); a
**cooldown**-gated `rally`/`sortie`; a `defend` action with `roll` + `if/then/else`
+ `compute`; and two **endings** decided by which fires first.

```bash
lono -d ./.lono game import --spec-file examples/last-watch.lono.json
# Hold the line (seed 42 → survive to scheduled relief at tick 3):
lono -d ./.lono play start last-watch --id hold --seed 42
lono -d ./.lono do hold arc defend
lono -d ./.lono advance hold 1     # ×3 — nightly drain each tick; relief fires at tick 3 → "relieved"
# Fall (drive morale/supplies down so the starvation → breach chain fires):
lono -d ./.lono play start last-watch --id fall --seed 7
```

### hollow-manor — locations, travel, spatial queries, lore/codex
Solve a murder by exploring the house. Shows **worlds & maps**: six `location`
entities each with authored prose `description`; an `exit` relationship graph (one
locked); `move` with `via:"exit"` for connected, guarded travel; **spatial derived**
queries that update live as you move (`exits_here`, and `here` = who's in the room);
a `clues` **set** that gates the accusation; and a **lore codex** (four entries with
`subject`/`tags`/`when`) revealed by `discover` as you examine rooms.

```bash
lono -d ./.lono game import --spec-file examples/hollow-manor.lono.json
lono -d ./.lono play start hollow-manor --id m1 --seed 17
lono -d ./.lono inspect m1 derived                 # exits_here / here update as you move
lono -d ./.lono lore list hollow-manor             # the authored world bible
#   examine rooms to collect clues + discover lore, travel via exits,
#   then accuse with enough evidence → "solved"
```
