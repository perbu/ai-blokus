# AI Blokus

A tool for evaluating LLM spatial reasoning by having them play Blokus against each other.

## Purpose

This is an experimentation and evaluation tool, not a polished game. The goal is to observe how different LLMs handle:
- Spatial reasoning on a 2D grid
- Rule compliance (diagonal adjacency, no edge sharing)
- Strategic planning with constrained placement
- Coordinate accuracy

## Architecture

```
game/       - Rules engine: pieces, board, move validation
player/     - LLM player: prompt construction, API calls, response parsing
tui/        - Bubble Tea terminal UI
debug/      - File-based debug logging
main.go     - CLI entry point, flag parsing
```

The game engine is fully independent of the LLM and TUI layers.

## Running

```bash
OPENROUTER_API_KEY=sk-or-... go run . --models "model1,model2,model3,model4"
```

Debug log writes to `debug.log` by default. Watch it live with `tail -f debug.log`.

## Key design decisions

- **One-shot moves with retry**: LLMs get one attempt, then one retry with the error message. Second failure = pass for that turn.
- **Pass is per-turn**: A player who passes (or fails) is re-evaluated for valid moves on their next turn. Only permanently out when the engine confirms no moves exist.
- **OpenRouter by default**: Enables easy cross-model comparison. Supports any OpenAI-compatible endpoint via `--base-url`.
- **Invalid moves are expected**: LLMs frequently produce illegal moves. The game handles this gracefully — log the error, give a retry, move on.

## Testing

```bash
go test ./game/ -v
```

Game engine tests cover piece orientation generation, move validation, and placement rules.
