package game

import "fmt"

// GameMode determines the board size and starting positions.
type GameMode int

const (
	ModeClassic GameMode = iota // 20x20, 2 or 4 players, corners
	ModeDuo                     // 14x14, 2 players, fixed start points
)

const (
	ClassicBoardSize = 20
	DuoBoardSize     = 14
)

// StartingCells returns the starting positions for each player in the given mode.
func StartingCells(mode GameMode, numPlayers int) map[int]Cell {
	if mode == ModeDuo {
		return map[int]Cell{
			1: {4, 4},
			2: {9, 9},
		}
	}
	// Classic: corners
	size := ClassicBoardSize
	return map[int]Cell{
		1: {0, 0},
		2: {0, size - 1},
		3: {size - 1, size - 1},
		4: {size - 1, 0},
	}
}

// PlayerState tracks a single player's state.
type PlayerState struct {
	ID              int
	Color           string
	StartCell       Cell
	Remaining       map[string]*Piece
	PiecesPlaced    int
	CellsRemaining  int
	LastPiecePlaced string
	Eliminated      bool // engine confirmed: no valid moves with any remaining piece
}

// LogEntry represents one entry in the game commentary log.
type LogEntry struct {
	PlayerID int
	Color    string
	Piece    string
	Comment  string
	IsPass   bool
	Error    string
}

// Game holds the complete game state.
type Game struct {
	Board       Board
	Players     []*PlayerState
	NumPlayers  int
	CurrentTurn int // index into Players (0-based)
	TurnNumber  int
	Log         []LogEntry
	GameOver    bool
	PieceMap    map[string]*Piece
	Mode        GameMode
}

// NewGame creates a new Blokus game with the specified mode and number of players.
func NewGame(mode GameMode, numPlayers int) *Game {
	colors := []string{"Red", "Blue", "Yellow", "Green"}
	allPieces := PieceMap()
	starts := StartingCells(mode, numPlayers)

	boardSize := ClassicBoardSize
	if mode == ModeDuo {
		boardSize = DuoBoardSize
	}

	players := make([]*PlayerState, numPlayers)
	for i := 0; i < numPlayers; i++ {
		remaining := make(map[string]*Piece)
		totalCells := 0
		for name, p := range allPieces {
			remaining[name] = p
			totalCells += p.Size
		}
		players[i] = &PlayerState{
			ID:             i + 1,
			Color:          colors[i],
			StartCell:      starts[i+1],
			Remaining:      remaining,
			CellsRemaining: totalCells,
		}
	}

	return &Game{
		Board:      NewBoard(boardSize),
		Players:    players,
		NumPlayers: numPlayers,
		PieceMap:   allPieces,
		Mode:       mode,
	}
}

// CurrentPlayer returns the player whose turn it is.
func (g *Game) CurrentPlayer() *PlayerState {
	return g.Players[g.CurrentTurn]
}

// ApplyMove validates and applies a move. Returns an error if invalid.
func (g *Game) ApplyMove(pieceName string, cells []Cell) error {
	player := g.CurrentPlayer()

	// Check piece is available
	piece, ok := player.Remaining[pieceName]
	if !ok {
		return fmt.Errorf("piece %s is not available", pieceName)
	}

	// Check cells match a valid orientation
	if matched, _ := piece.MatchesOrientation(cells); !matched {
		return fmt.Errorf("cells don't match any orientation of piece %s", pieceName)
	}

	// Check placement is legal
	isFirst := player.PiecesPlaced == 0
	if err := g.Board.ValidatePlacement(player.ID, cells, isFirst, player.StartCell); err != nil {
		return err
	}

	// Apply
	g.Board.Place(player.ID, cells)
	delete(player.Remaining, pieceName)
	player.PiecesPlaced++
	player.CellsRemaining -= piece.Size
	player.LastPiecePlaced = pieceName

	return nil
}

// Eliminate marks the current player as permanently out (no valid moves).
func (g *Game) Eliminate() {
	g.CurrentPlayer().Eliminated = true
}

// AdvanceTurn moves to the next player and checks for game over.
func (g *Game) AdvanceTurn() {
	g.TurnNumber++
	g.CurrentTurn = (g.CurrentTurn + 1) % g.NumPlayers

	allOut := true
	for _, p := range g.Players {
		if !p.Eliminated {
			allOut = false
			break
		}
	}
	if allOut {
		g.GameOver = true
	}
}

// RemainingPieces returns the remaining pieces for a player as a sorted slice.
func (ps *PlayerState) RemainingPieceNames() []string {
	// Return in a consistent order
	order := []string{
		"O1", "I2", "I3", "L3",
		"I4", "O4", "T4", "S4", "L4",
		"F5", "I5", "L5", "N5", "P5", "T5", "U5", "V5", "W5", "X5", "Y5", "Z5",
	}
	var names []string
	for _, name := range order {
		if _, ok := ps.Remaining[name]; ok {
			names = append(names, name)
		}
	}
	return names
}

// RemainingPieceSlice returns remaining Piece objects.
func (ps *PlayerState) RemainingPieceSlice() []*Piece {
	names := ps.RemainingPieceNames()
	pieces := make([]*Piece, len(names))
	for i, name := range names {
		pieces[i] = ps.Remaining[name]
	}
	return pieces
}

// Score calculates the player's score. Lower (more negative) is worse.
func (ps *PlayerState) Score() int {
	score := -ps.CellsRemaining
	if ps.CellsRemaining == 0 {
		score += 15
		if ps.LastPiecePlaced == "O1" {
			score += 5
		}
	}
	return score
}

// CheckPlayerCanMove returns true if the current player has any valid move.
// If not, marks them as eliminated.
func (g *Game) CheckPlayerCanMove() bool {
	player := g.CurrentPlayer()
	if player.Eliminated {
		return false
	}
	isFirst := player.PiecesPlaced == 0
	pieces := player.RemainingPieceSlice()
	if !g.Board.HasValidMove(player.ID, pieces, isFirst, player.StartCell) {
		player.Eliminated = true
		return false
	}
	return true
}
