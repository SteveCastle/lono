# lono — Claude Code plugin

Collaboratively **build** and **run** story-driven, relationship-driven games with the
[`lono`](https://github.com/callsignmedia/lono) state engine, right inside Claude
Code. The engine enforces the rules (types, bounds, references, action guards),
so the model narrates while lono keeps the game consistent.

## What's inside

- **`creating-a-game`** skill — collaboratively design a game (premise,
  characters, relationships, arc, branching points, endings, items, beats) and
  build a validated, portable game definition you can save to a `.lono.json`
  file.
- **`running-a-game`** skill — load a saved game and play it turn-by-turn: the
  model reads the current state and legal actions from the engine, narrates the
  scene, offers choices, applies them, and handles endings and save points.
- **Bundled engine** — prebuilt `lono` binaries for macOS (arm64/amd64), Linux
  (amd64/arm64), and Windows (amd64) under `bin/`, selected automatically by a
  small wrapper. No Go toolchain required.

## Install

From a checkout of this repo (the marketplace is the `plugin/` directory):

```
/plugin marketplace add /path/to/lono/plugin
/plugin install lono@lono-marketplace
```

Or test it locally without installing:

```
claude --plugin-dir /path/to/lono/plugin/lono
```

Once enabled, the skills activate automatically when you ask to build or play a
game, or invoke them explicitly: `/lono:creating-a-game`, `/lono:running-a-game`.

## Quick start

> "Let's make a short branching story set at an art gallery."

The `creating-a-game` skill walks you through the design, builds the definition,
validates it, and saves `your-game.lono.json`.

> "Play my-game.lono.json"

The `running-a-game` skill loads it, starts a session, and game-masters it.

## How the engine is invoked

`bin/` is added to the Bash tool's PATH while the plugin is enabled, so the
skills simply call `lono …`. The `bin/lono` wrapper picks the matching binary for
your OS/arch and `exec`s it. State is stored under the data dir the skills pass
(`--data-dir ./.lono` by default). Every command prints a JSON envelope; see each
skill's `reference.md` for the full command and output details.

## Updating the bundled engine

The binaries are cross-compiled from this repo's engine source:

```
GOOS=darwin  GOARCH=arm64 go build -o plugin/lono/bin/lono-darwin-arm64   ./cmd/lono
GOOS=darwin  GOARCH=amd64 go build -o plugin/lono/bin/lono-darwin-amd64   ./cmd/lono
GOOS=linux   GOARCH=amd64 go build -o plugin/lono/bin/lono-linux-amd64    ./cmd/lono
GOOS=linux   GOARCH=arm64 go build -o plugin/lono/bin/lono-linux-arm64    ./cmd/lono
GOOS=windows GOARCH=amd64 go build -o plugin/lono/bin/lono-windows-amd64.exe ./cmd/lono
```
