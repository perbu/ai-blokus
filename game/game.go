package game

import "fmt"

// PlayerState tracks a single player's state.
type PlayerState struct {
	ID              int
	Color           string
	Corner          Cell
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
}

// NewGame creates a new Blokus game with the specified number of players.
func NewGame(numPlayers int) *Game {
	colors := []string{"Red", "Blue", "Yellow", "Green"}
	allPieces := PieceMap()

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
			Corner:         PlayerCorners[i+1],
			Remaining:      remaining,
			CellsRemaining: totalCells,
		}
	}

	return &Game{
		Players:    players,
		NumPlayers: numPlayers,
		PieceMap:   allPieces,
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
	if err := g.Board.ValidatePlacement(player.ID, cells, isFirst); err != nil {
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
	if !g.Board.HasValidMove(player.ID, pieces, isFirst) {
		player.Eliminated = true
		return false
	}
	return true
}
