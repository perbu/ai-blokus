package game

import (
	"fmt"
	"strings"
)

// Board represents the Blokus game board with configurable size.
// Cell values: 0 = empty, 1-4 = player ID.
type Board struct {
	Size int
	Grid [][]int
}

// NewBoard creates a board of the given size.
func NewBoard(size int) Board {
	grid := make([][]int, size)
	for i := range grid {
		grid[i] = make([]int, size)
	}
	return Board{Size: size, Grid: grid}
}

// IsEmpty returns true if the cell is unoccupied.
func (b *Board) IsEmpty(r, c int) bool {
	return b.InBounds(r, c) && b.Grid[r][c] == 0
}

// InBounds returns true if (r,c) is within the board.
func (b *Board) InBounds(r, c int) bool {
	return r >= 0 && r < b.Size && c >= 0 && c < b.Size
}

// Place puts a player's piece on the board. No validation.
func (b *Board) Place(playerID int, cells []Cell) {
	for _, c := range cells {
		b.Grid[c.Row][c.Col] = playerID
	}
}

// ValidatePlacement checks if placing cells for playerID is legal.
// isFirst indicates whether this is the player's first piece.
// startCell is the cell the first piece must cover (player's starting position).
func (b *Board) ValidatePlacement(playerID int, cells []Cell, isFirst bool, startCell Cell) error {
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
			if b.InBounds(nr, nc) && b.Grid[nr][nc] == playerID {
				return fmt.Errorf("cell (%d,%d) shares an edge with your piece at (%d,%d)", c.Row, c.Col, nr, nc)
			}
		}
	}

	if isFirst {
		// First piece must cover the player's starting cell
		coversStart := false
		for _, c := range cells {
			if c.Row == startCell.Row && c.Col == startCell.Col {
				coversStart = true
				break
			}
		}
		if !coversStart {
			return fmt.Errorf("first piece must cover your starting position (%d,%d)", startCell.Row, startCell.Col)
		}
	} else {
		// Subsequent pieces must touch at least one diagonal of own piece
		hasDiagonal := false
		for _, c := range cells {
			for _, d := range [][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}} {
				nr, nc := c.Row+d[0], c.Col+d[1]
				if b.InBounds(nr, nc) && b.Grid[nr][nc] == playerID {
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
func (b *Board) HasValidMove(playerID int, pieces []*Piece, isFirst bool, startCell Cell) bool {
	for _, piece := range pieces {
		for _, orient := range piece.Orientations {
			for r := 0; r < b.Size; r++ {
				for c := 0; c < b.Size; c++ {
					cells := translateCells(orient, r, c)
					if b.ValidatePlacement(playerID, cells, isFirst, startCell) == nil {
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

	for r := 0; r < b.Size; r++ {
		for c := 0; c < b.Size; c++ {
			if b.Grid[r][c] != playerID {
				continue
			}
			for _, d := range [][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}} {
				nr, nc := r+d[0], c+d[1]
				cell := Cell{nr, nc}
				if !b.InBounds(nr, nc) || b.Grid[nr][nc] != 0 || seen[cell] {
					continue
				}
				edgeAdj := false
				for _, e := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
					er, ec := nr+e[0], nc+e[1]
					if b.InBounds(er, ec) && b.Grid[er][ec] == playerID {
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
	for c := 0; c < b.Size; c++ {
		fmt.Fprintf(&sb, "%3d", c)
	}
	sb.WriteString("\n")

	for r := 0; r < b.Size; r++ {
		fmt.Fprintf(&sb, "%2d ", r)
		for c := 0; c < b.Size; c++ {
			sb.WriteString(symbols[b.Grid[r][c]])
			sb.WriteString(" ")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
