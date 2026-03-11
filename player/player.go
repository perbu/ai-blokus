package player

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/perbu/ai-blokus/debug"
	"github.com/perbu/ai-blokus/game"
	openai "github.com/sashabaranov/go-openai"
)

// Move represents a player's move response.
type Move struct {
	Piece       string  `json:"piece"`
	Anchor      []int   `json:"anchor,omitempty"`      // [row, col] from LLM
	Orientation int     `json:"orientation"`            // orientation index from LLM
	Pass        bool    `json:"pass"`
	Comment     string  `json:"comment"`
	Cells       [][]int `json:"-"` // computed from anchor + orientation, not from JSON
}

// Player can produce moves given a game state.
type Player interface {
	GetMove(ctx context.Context, g *game.Game) (*Move, error)
	Name() string
	Model() string
}

// LLMPlayer uses an OpenAI-compatible API to play.
type LLMPlayer struct {
	client      *openai.Client
	model       string
	playerID    int
	name        string
	OnToken func() // called on every received chunk (for token counting)
}

// NewLLMPlayer creates a new LLM-backed player.
func NewLLMPlayer(apiKey, baseURL, model, name string, playerID int) *LLMPlayer {
	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}
	return &LLMPlayer{
		client:   openai.NewClientWithConfig(config),
		model:    model,
		playerID: playerID,
		name:     name,
	}
}

func (p *LLMPlayer) Name() string  { return p.name }
func (p *LLMPlayer) Model() string { return p.model }

// GetMove requests a move from the LLM. On failure, retries up to 2 times with the error.
func (p *LLMPlayer) GetMove(ctx context.Context, g *game.Game) (*Move, error) {
	const maxRetries = 2
	turn := g.TurnNumber + 1

	debug.Log("[turn %d] [%s/%s] requesting move", turn, p.name, p.model)

	move, lastErr := p.tryMove(ctx, g, nil)
	if lastErr == nil {
		return move, nil
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		debug.Log("[turn %d] [%s/%s] retry %d/%d after error: %v", turn, p.name, p.model, attempt, maxRetries, lastErr)
		move, lastErr = p.tryMove(ctx, g, lastErr)
		if lastErr == nil {
			return move, nil
		}
	}

	debug.Log("[turn %d] [%s/%s] all %d retries failed: %v", turn, p.name, p.model, maxRetries, lastErr)
	return move, lastErr
}

// tryMove makes one API call. Returns (move, validationError).
// An API or parse error is returned as a regular error via (nil, err).
// A validation error returns the parsed move AND the error, so the caller
// can retry or log the attempt.
func (p *LLMPlayer) tryMove(ctx context.Context, g *game.Game, prevErr error) (*Move, error) {
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt(g.Board.Size)},
		{Role: openai.ChatMessageRoleUser, Content: p.buildPrompt(g)},
	}

	if prevErr != nil {
		messages = append(messages,
			openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "(previous invalid attempt)",
			},
			openai.ChatCompletionMessage{
				Role: openai.ChatMessageRoleUser,
				Content: fmt.Sprintf(
					"Your previous move was INVALID: %s\n\nPlease try again. Double-check that:\n"+
						"1. The piece name is one of your remaining pieces\n"+
						"2. The orientation index is valid for that piece\n"+
						"3. The anchor places all cells on the board (0-%d) and on empty squares\n"+
						"4. At least one placed cell is a diagonal touch point\n"+
						"5. No placed cell is orthogonally adjacent to your existing pieces\n\n"+
						"Respond with corrected JSON only.", prevErr, g.Board.Size-1),
			},
		)
	}

	if prevErr != nil {
		retryMsg := messages[len(messages)-1].Content
		debug.Log("[%s/%s] sending retry prompt (%d chars):\n%s",
			p.name, p.model, len(retryMsg), retryMsg)
	} else {
		prompt := messages[1].Content
		debug.Log("[%s/%s] sending prompt (%d chars):\n%s",
			p.name, p.model, len(prompt), prompt)
	}

	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:       p.model,
		Messages:    messages,
		Temperature: 0.7,
	})
	if err != nil {
		debug.Log("[%s/%s] API error: %v", p.name, p.model, err)
		return nil, fmt.Errorf("API call failed: %w", err)
	}
	defer stream.Close()

	var contentBuilder strings.Builder
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			debug.Log("[%s/%s] stream error: %v", p.name, p.model, err)
			return nil, fmt.Errorf("stream error: %w", err)
		}
		if len(chunk.Choices) > 0 {
			if p.OnToken != nil {
				p.OnToken()
			}
			contentBuilder.WriteString(chunk.Choices[0].Delta.Content)
		}
	}

	content := contentBuilder.String()
	if content == "" {
		debug.Log("[%s/%s] empty response (no content)", p.name, p.model)
		return nil, fmt.Errorf("no response from API")
	}
	debug.Log("[%s/%s] raw response:\n%s", p.name, p.model, content)

	move, err := parseMove(content)
	if err != nil {
		debug.Log("[%s/%s] parse error: %v", p.name, p.model, err)
		return nil, err
	}

	debug.Log("[%s/%s] parsed move: piece=%s pass=%v anchor=%v orientation=%d comment=%q",
		p.name, p.model, move.Piece, move.Pass, move.Anchor, move.Orientation, move.Comment)

	// Resolve and validate the move
	if !move.Pass {
		if err := p.resolveAndValidate(g, move); err != nil {
			return move, err
		}
	}

	return move, nil
}

// resolveAndValidate converts anchor+orientation into absolute cells and validates.
func (p *LLMPlayer) resolveAndValidate(g *game.Game, move *Move) error {
	player := g.Players[p.playerID-1]

	piece, ok := player.Remaining[move.Piece]
	if !ok {
		return fmt.Errorf("piece %s is not available", move.Piece)
	}

	if move.Orientation < 0 || move.Orientation >= len(piece.Orientations) {
		return fmt.Errorf("orientation %d invalid for piece %s (valid: 0-%d)",
			move.Orientation, move.Piece, len(piece.Orientations)-1)
	}

	if len(move.Anchor) != 2 {
		return fmt.Errorf("anchor must be [row, col], got %v", move.Anchor)
	}

	// Compute absolute cells from anchor + orientation
	orient := piece.Orientations[move.Orientation]
	cells := make([][]int, len(orient))
	for i, c := range orient {
		cells[i] = []int{c.Row + move.Anchor[0], c.Col + move.Anchor[1]}
	}
	move.Cells = cells

	debug.Log("[%s/%s] resolved cells: anchor=%v + orientation %d → %v",
		p.name, p.model, move.Anchor, move.Orientation, cells)

	// Validate using the game engine
	gameCells := move.ToCells()
	isFirst := player.PiecesPlaced == 0
	return g.Board.ValidatePlacement(player.ID, gameCells, isFirst, player.StartCell)
}

func systemPrompt(boardSize int) string {
	maxIdx := boardSize - 1
	return fmt.Sprintf(`You are an expert Blokus player. Output ONLY a JSON object. No explanation, no reasoning, no other text.

To place a piece:
{"piece":"PIECE_NAME","anchor":[row,col],"orientation":INDEX,"comment":"..."}

To pass (only when you have no valid moves):
{"pass":true,"comment":"..."}

HOW MOVES WORK:
- Pick a piece from your remaining pieces
- Pick an orientation by its index number (each piece lists its orientations with cell offsets)
- Pick an anchor point [row, col] — each cell offset in the orientation is ADDED to the anchor to get the board position
- Formula: board_cell = (anchor_row + offset_row, anchor_col + offset_col)
- REVERSE: to place orientation cell (dr,dc) onto board cell (R,C), set anchor to [R-dr, C-dc]

EXAMPLE (forward): orientation offsets (0,0),(0,1),(1,0) with anchor [3,5] → placed at (3,5),(3,6),(4,5)
EXAMPLE (reverse): you need to cover board cell (0,%d). Orientation has a cell at offset (0,2). Set anchor to [0-0, %d-2] = [0,%d]. Then (0,2)+[0,%d] = (0,%d). ✓

IMPORTANT: All placed cells must be within bounds (rows 0-%d, cols 0-%d). If your piece has 3 rows and you anchor at row %d, row offsets 0,1,2 give rows %d,%d,%d — row %d is OUT OF BOUNDS. Adjust the anchor so all cells stay on the board.

RULES:
- Your FIRST piece must cover your starting position cell
- Every subsequent piece must touch at least one of your existing pieces DIAGONALLY (corner-to-corner)
- Your pieces must NEVER share an edge (horizontally/vertically adjacent) with your own pieces
- Your pieces CAN share edges with opponents' pieces
- Every cell must be empty (shown as . on the board)

The prompt shows DIAGONAL TOUCH POINTS — at least one cell of your placed piece must land on one of these positions. Use them to guide your anchor choice.`,
		maxIdx, maxIdx, maxIdx-2, maxIdx-2, maxIdx,
		maxIdx, maxIdx,
		maxIdx-1, maxIdx-1, maxIdx, maxIdx+1, maxIdx+1)
}

func (p *LLMPlayer) buildPrompt(g *game.Game) string {
	player := g.Players[p.playerID-1]
	var sb strings.Builder

	fmt.Fprintf(&sb, "You are %s (Player %d). Your starting position is (%d,%d).\n",
		player.Color, player.ID, player.StartCell.Row, player.StartCell.Col)

	if player.PiecesPlaced == 0 {
		fmt.Fprintf(&sb, "This is your FIRST move. Your piece MUST cover cell (%d,%d).\n",
			player.StartCell.Row, player.StartCell.Col)
		fmt.Fprintf(&sb, "Choose an orientation, then set anchor so that one cell lands on (%d,%d).\n",
			player.StartCell.Row, player.StartCell.Col)
	} else {
		// Show attachment points
		points := g.Board.AttachmentPoints(player.ID)
		sb.WriteString("\nDIAGONAL TOUCH POINTS (at least one cell of your piece must land on one of these):\n  ")
		for i, pt := range points {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "(%d,%d)", pt.Row, pt.Col)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nBOARD STATE (. = empty, R/B/Y/G = player pieces):\n")
	sb.WriteString(g.Board.Serialize())

	sb.WriteString("\nYOUR REMAINING PIECES:\n")
	for _, name := range player.RemainingPieceNames() {
		piece := player.Remaining[name]
		fmt.Fprintf(&sb, "  %s (%d cells, %d orientations):\n", name, piece.Size, len(piece.Orientations))
		for i, orient := range piece.Orientations {
			fmt.Fprintf(&sb, "    #%d: %s  offsets: %s\n", i, game.RenderCells(orient), game.RenderCellOffsets(orient))
		}
	}

	if len(g.Log) > 0 {
		sb.WriteString("\nRECENT MOVES:\n")
		start := 0
		if len(g.Log) > 8 {
			start = len(g.Log) - 8
		}
		for _, entry := range g.Log[start:] {
			if entry.Error != "" {
				fmt.Fprintf(&sb, "  %s: invalid move (%s)\n", entry.Color, entry.Error)
			} else if entry.IsPass {
				fmt.Fprintf(&sb, "  %s: passed\n", entry.Color)
			} else {
				fmt.Fprintf(&sb, "  %s: placed %s — %s\n", entry.Color, entry.Piece, entry.Comment)
			}
		}
	}

	return sb.String()
}

func parseMove(content string) (*Move, error) {
	content = strings.TrimSpace(content)
	// Strip markdown code fences if present
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) >= 3 {
			content = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	content = strings.TrimSpace(content)

	// Try parsing the whole content as JSON first
	var move Move
	if err := json.Unmarshal([]byte(content), &move); err == nil {
		return &move, nil
	}

	// Extract the last JSON object from the response (models often prepend reasoning text)
	lastOpen := strings.LastIndex(content, "{")
	lastClose := strings.LastIndex(content, "}")
	if lastOpen >= 0 && lastClose > lastOpen {
		jsonStr := content[lastOpen : lastClose+1]
		if err := json.Unmarshal([]byte(jsonStr), &move); err == nil {
			return &move, nil
		}
	}

	return nil, fmt.Errorf("failed to parse move JSON: no valid JSON object found\nRaw response: %s", content)
}

// ToCells converts the [[row,col],...] format to game.Cell slice.
func (m *Move) ToCells() []game.Cell {
	cells := make([]game.Cell, len(m.Cells))
	for i, rc := range m.Cells {
		if len(rc) >= 2 {
			cells[i] = game.Cell{Row: rc[0], Col: rc[1]}
		}
	}
	return cells
}
