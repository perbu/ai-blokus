# AI Blokus

LLMs playing Blokus against each other in the terminal.

Four AI players compete on a 20×20 board, placing polyomino pieces while
commenting on their strategy. Uses [OpenRouter](https://openrouter.ai) to pit
different models against each other.

## Setup

```bash
cp .env.example .env
# Edit .env with your OpenRouter API key
```

## Run

Same model for all players:

```bash
go run . --model anthropic/claude-sonnet-4.6
```

Four different models battling it out:

```bash
go run . --models "anthropic/claude-sonnet-4.6,openai/gpt-4o,google/gemini-3-flash-preview,deepseek/deepseek-v3.2"
```

Two-player game:

```bash
go run . --players 2 --models "anthropic/claude-sonnet-4.6,openai/gpt-4o"
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--players` | `4` | Number of players (2 or 4) |
| `--model` | `openai/gpt-4o-mini` | Model for all players |
| `--models` | | Comma-separated model per player |
| `--base-url` | OpenRouter | API endpoint override |

## Environment variables

| Variable | Description |
|----------|-------------|
| `OPENROUTER_API_KEY` | OpenRouter API key (preferred) |
| `OPENAI_API_KEY` | Fallback API key |
| `OPENAI_BASE_URL` | Override API endpoint |

These can be set in a `.env` file in the project root.

## How it works

Each turn, the current player's LLM receives:
- The full board state as an ASCII grid
- Its remaining pieces with shape diagrams
- Recent move history with player commentary

The LLM responds with a piece name, board coordinates, and a strategic comment.
Invalid moves result in an automatic pass. The game ends when no player can
place a piece.

## Controls

Press `q` to quit during a game.

## Trademark notice

Blokus is a trademark of SEKKOIA S.A.S. This project implements the game
mechanics for AI evaluation purposes only and is not designed for human play.
