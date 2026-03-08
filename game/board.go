package game

import (
	"fmt"
	"strings"
)

const BoardSize = 20

// Board represents the 20x20 Blokus game board.
// Cell values: 0 = empty, 1-4 = player ID.
type Board [BoardSize][BoardSize]int

// PlayerCorners returns the starting corner for each player (1-indexed).
var PlayerCorners = map[int]Cell{
	1: {0, 0},
	2: {0, BoardSize - 1},
	3: {BoardSize - 1, BoardSize - 1},
	4: {BoardSize - 1, 0},
}

// IsEmpty returns true if the cell is unoccupied.
func (b *Board) IsEmpty(r, c int) bool {
	return b.InBounds(r, c) && b[r][c] == 0
}

// InBounds returns true if (r,c) is within the board.
func (b *Board) InBounds(r, c int) bool {
	return r >= 0 && r < BoardSize && c >= 0 && c < BoardSize
}

// Place puts a player's piece on the board. No validation.
func (b *Board) Place(playerID int, cells []Cell) {
	for _, c := range cells {
		b[c.Row][c.Col] = playerID
	}
}

// ValidatePlacement checks if placing cells for playerID is legal.
// isFirst indicates whether this is the player's first piece.
func (b *Board) ValidatePlacement(playerID int, cells []Cell, isFirst bool) error {
	if len(cells) == 0 {
		return fmt.Errorf("no cells specified")
	}

	// Check all cells are on board and empty
	for _, c := range cells {
		if !b.InBounds(c.Row, c.Col) {
			return fmt.Errorf("cell (%d,%d) is out of bounds", c.Row, c.Col)
		}
		if !b.IsEmpty(c.Row, c.Col) {
			return fmt.Errorf("cell (%d,%d) is already occupied", c.Row, c.Col)
		}
	}

	// Check no edge-adjacency with own pieces
	for _, c := range cells {
		for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
			nr, nc := c.Row+d[0], c.Col+d[1]
			if b.InBounds(nr, nc) && b[nr][nc] == playerID {
				return fmt.Errorf("cell (%d,%d) shares an edge with your piece at (%d,%d)", c.Row, c.Col, nr, nc)
			}
		}
	}

	if isFirst {
		// First piece must cover the player's starting corner
		corner := PlayerCorners[playerID]
		coversCorner := false
		for _, c := range cells {
			if c.Row == corner.Row && c.Col == corner.Col {
				coversCorner = true
				break
			}
		}
		if !coversCorner {
			return fmt.Errorf("first piece must cover your starting corner (%d,%d)", corner.Row, corner.Col)
		}
	} else {
		// Subsequent pieces must touch at least one diagonal of own piece
		hasDiagonal := false
		for _, c := range cells {
			for _, d := range [][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}} {
				nr, nc := c.Row+d[0], c.Col+d[1]
				if b.InBounds(nr, nc) && b[nr][nc] == playerID {
					hasDiagonal = true
					break
				}
			}
			if hasDiagonal {
				break
			}
		}
		if !hasDiagonal {
			return fmt.Errorf("piece must diagonally touch one of your existing pieces")
		}
	}

	return nil
}

// HasValidMove checks if the player has any valid placement for any of their remaining pieces.
func (b *Board) HasValidMove(playerID int, pieces []*Piece, isFirst bool) bool {
	for _, piece := range pieces {
		for _, orient := range piece.Orientations {
			for r := 0; r < BoardSize; r++ {
				for c := 0; c < BoardSize; c++ {
					cells := translateCells(orient, r, c)
					if b.ValidatePlacement(playerID, cells, isFirst) == nil {
						return true
					}
				}
			}
		}
	}
	return false
}

func translateCells(orient []Cell, dr, dc int) []Cell {
	out := make([]Cell, len(orient))
	for i, c := range orient {
		out[i] = Cell{c.Row + dr, c.Col + dc}
	}
	return out
}

// AttachmentPoints returns empty cells where a player could satisfy the diagonal
// touch rule: diagonally adjacent to at least one of their pieces and not
// orthogonally adjacent to any of their pieces.
func (b *Board) AttachmentPoints(playerID int) []Cell {
	var points []Cell
	seen := make(map[Cell]bool)

	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			if b[r][c] != playerID {
				continue
			}
			for _, d := range [][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}} {
				nr, nc := r+d[0], c+d[1]
				cell := Cell{nr, nc}
				if !b.InBounds(nr, nc) || b[nr][nc] != 0 || seen[cell] {
					continue
				}
				edgeAdj := false
				for _, e := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
					er, ec := nr+e[0], nc+e[1]
					if b.InBounds(er, ec) && b[er][ec] == playerID {
						edgeAdj = true
						break
					}
				}
				if !edgeAdj {
					seen[cell] = true
					points = append(points, cell)
				}
			}
		}
	}
	return points
}

// Serialize returns a text representation of the board for display or LLM prompts.
func (b *Board) Serialize() string {
	symbols := map[int]string{0: " .", 1: " R", 2: " B", 3: " Y", 4: " G"}
	var sb strings.Builder

	// Header
	sb.WriteString("   ")
	for c := 0; c < BoardSize; c++ {
		fmt.Fprintf(&sb, "%3d", c)
	}
	sb.WriteString("\n")

	for r := 0; r < BoardSize; r++ {
		fmt.Fprintf(&sb, "%2d ", r)
		for c := 0; c < BoardSize; c++ {
			sb.WriteString(symbols[b[r][c]])
			sb.WriteString(" ")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
