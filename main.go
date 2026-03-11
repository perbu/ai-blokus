package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"

	tea "charm.land/bubbletea/v2"
	"github.com/perbu/ai-blokus/debug"
	"github.com/perbu/ai-blokus/game"
	"github.com/perbu/ai-blokus/player"
	"github.com/perbu/ai-blokus/tui"
)

const defaultBaseURL = "https://openrouter.ai/api/v1"

func main() {
	_ = godotenv.Load() // silently ignore if .env doesn't exist

	debugLog := flag.String("debug", "debug.log", "debug log file path (empty to disable)")
	numPlayers := flag.Int("players", 4, "number of players (2 or 4)")
	duo := flag.Bool("duo", false, "play Blokus Duo (14x14 board, 2 players)")
	model := flag.String("model", "openai/gpt-4o-mini", "model for all players (OpenRouter format)")
	models := flag.String("models", "", "comma-separated models per player (e.g. openai/gpt-4o,anthropic/claude-sonnet-4,google/gemini-2.0-flash,openai/gpt-4o-mini)")
	baseURL := flag.String("base-url", "", "API base URL (default: OpenRouter)")
	flag.Parse()

	if *debugLog != "" {
		if err := debug.Init(*debugLog); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open debug log: %v\n", err)
			os.Exit(1)
		}
		defer debug.Close()
	}

	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		// Fall back to OPENAI_API_KEY for direct OpenAI use
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENROUTER_API_KEY (or OPENAI_API_KEY) environment variable is required")
		os.Exit(1)
	}

	endpoint := defaultBaseURL
	if *baseURL != "" {
		endpoint = *baseURL
	} else if bu := os.Getenv("OPENAI_BASE_URL"); bu != "" {
		endpoint = bu
	}

	mode := game.ModeClassic
	if *duo {
		mode = game.ModeDuo
		*numPlayers = 2
	}

	if *numPlayers != 2 && *numPlayers != 4 {
		fmt.Fprintln(os.Stderr, "number of players must be 2 or 4")
		os.Exit(1)
	}

	if mode == game.ModeDuo && *numPlayers != 2 {
		fmt.Fprintln(os.Stderr, "Blokus Duo requires exactly 2 players")
		os.Exit(1)
	}

	// Resolve per-player models
	playerModels := make([]string, *numPlayers)
	if *models != "" {
		parts := strings.Split(*models, ",")
		if len(parts) != *numPlayers {
			fmt.Fprintf(os.Stderr, "--models must have exactly %d comma-separated values\n", *numPlayers)
			os.Exit(1)
		}
		for i, m := range parts {
			playerModels[i] = strings.TrimSpace(m)
		}
	} else {
		for i := range playerModels {
			playerModels[i] = *model
		}
	}

	g := game.NewGame(mode, *numPlayers)

	colors := []string{"Red", "Blue", "Yellow", "Green"}
	players := make([]player.Player, *numPlayers)
	for i := 0; i < *numPlayers; i++ {
		players[i] = player.NewLLMPlayer(apiKey, endpoint, playerModels[i], colors[i], i+1)
	}

	variant := "Blokus"
	if mode == game.ModeDuo {
		variant = "Blokus Duo"
	}
	debug.Log("starting %d-player %s game via %s", *numPlayers, variant, endpoint)
	for i, m := range playerModels {
		debug.Log("  player %d (%s): %s", i+1, colors[i], m)
	}

	fmt.Fprintf(os.Stderr, "Starting %d-player %s via %s\n", *numPlayers, variant, endpoint)
	for i, m := range playerModels {
		fmt.Fprintf(os.Stderr, "  %s: %s\n", colors[i], m)
	}

	m := tui.New(g, players)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
