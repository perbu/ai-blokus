package game

import (
	"fmt"
	"sort"
	"strings"
)

// Cell represents a position on the board or within a piece shape.
type Cell struct {
	Row, Col int
}

// Piece represents a Blokus piece with all its unique orientations.
type Piece struct {
	Name        string
	Size        int
	Orientations [][]Cell // each orientation is a normalized set of cells
}

// AllPieces returns the 21 standard Blokus pieces with precomputed orientations.
func AllPieces() []*Piece {
	defs := []struct {
		name  string
		cells []Cell
	}{
		// Monomino
		{"O1", []Cell{{0, 0}}},
		// Domino
		{"I2", []Cell{{0, 0}, {0, 1}}},
		// Trominoes
		{"I3", []Cell{{0, 0}, {0, 1}, {0, 2}}},
		{"L3", []Cell{{0, 0}, {0, 1}, {1, 0}}},
		// Tetrominoes
		{"I4", []Cell{{0, 0}, {0, 1}, {0, 2}, {0, 3}}},
		{"O4", []Cell{{0, 0}, {0, 1}, {1, 0}, {1, 1}}},
		{"T4", []Cell{{0, 0}, {0, 1}, {0, 2}, {1, 1}}},
		{"S4", []Cell{{0, 0}, {0, 1}, {1, 1}, {1, 2}}},
		{"L4", []Cell{{0, 0}, {1, 0}, {2, 0}, {2, 1}}},
		// Pentominoes
		{"F5", []Cell{{0, 1}, {0, 2}, {1, 0}, {1, 1}, {2, 1}}},
		{"I5", []Cell{{0, 0}, {0, 1}, {0, 2}, {0, 3}, {0, 4}}},
		{"L5", []Cell{{0, 0}, {1, 0}, {2, 0}, {3, 0}, {3, 1}}},
		{"N5", []Cell{{0, 0}, {1, 0}, {1, 1}, {2, 1}, {3, 1}}},
		{"P5", []Cell{{0, 0}, {0, 1}, {1, 0}, {1, 1}, {2, 0}}},
		{"T5", []Cell{{0, 0}, {0, 1}, {0, 2}, {1, 1}, {2, 1}}},
		{"U5", []Cell{{0, 0}, {0, 2}, {1, 0}, {1, 1}, {1, 2}}},
		{"V5", []Cell{{0, 0}, {1, 0}, {2, 0}, {2, 1}, {2, 2}}},
		{"W5", []Cell{{0, 0}, {1, 0}, {1, 1}, {2, 1}, {2, 2}}},
		{"X5", []Cell{{0, 1}, {1, 0}, {1, 1}, {1, 2}, {2, 1}}},
		{"Y5", []Cell{{0, 1}, {1, 0}, {1, 1}, {2, 1}, {3, 1}}},
		{"Z5", []Cell{{0, 0}, {1, 0}, {1, 1}, {1, 2}, {2, 2}}},
	}

	pieces := make([]*Piece, len(defs))
	for i, d := range defs {
		pieces[i] = &Piece{
			Name:        d.name,
			Size:        len(d.cells),
			Orientations: generateOrientations(d.cells),
		}
	}
	return pieces
}

// PieceMap returns a map from piece name to Piece.
func PieceMap() map[string]*Piece {
	m := make(map[string]*Piece)
	for _, p := range AllPieces() {
		m[p.Name] = p
	}
	return m
}

func generateOrientations(base []Cell) [][]Cell {
	seen := make(map[string]bool)
	var orientations [][]Cell

	current := base
	for flip := 0; flip < 2; flip++ {
		for rot := 0; rot < 4; rot++ {
			norm := normalize(current)
			key := cellsKey(norm)
			if !seen[key] {
				seen[key] = true
				orientations = append(orientations, norm)
			}
			current = rotate90(current)
		}
		current = flipH(base)
	}
	return orientations
}

// rotate90 rotates cells 90 degrees clockwise: (r,c) -> (c, -r)
func rotate90(cells []Cell) []Cell {
	out := make([]Cell, len(cells))
	for i, c := range cells {
		out[i] = Cell{c.Col, -c.Row}
	}
	return out
}

// flipH flips cells horizontally: (r,c) -> (r, -c)
func flipH(cells []Cell) []Cell {
	out := make([]Cell, len(cells))
	for i, c := range cells {
		out[i] = Cell{c.Row, -c.Col}
	}
	return out
}

// normalize translates cells so min row and col are 0, then sorts.
func normalize(cells []Cell) []Cell {
	if len(cells) == 0 {
		return cells
	}
	minR, minC := cells[0].Row, cells[0].Col
	for _, c := range cells[1:] {
		if c.Row < minR {
			minR = c.Row
		}
		if c.Col < minC {
			minC = c.Col
		}
	}
	out := make([]Cell, len(cells))
	for i, c := range cells {
		out[i] = Cell{c.Row - minR, c.Col - minC}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Row != out[j].Row {
			return out[i].Row < out[j].Row
		}
		return out[i].Col < out[j].Col
	})
	return out
}

func cellsKey(cells []Cell) string {
	parts := make([]string, len(cells))
	for i, c := range cells {
		parts[i] = fmt.Sprintf("%d,%d", c.Row, c.Col)
	}
	return strings.Join(parts, ";")
}

// MatchesOrientation checks if the given absolute cells match any orientation
// of this piece. Returns true and the orientation index if matched.
func (p *Piece) MatchesOrientation(cells []Cell) (bool, int) {
	norm := normalize(cells)
	key := cellsKey(norm)
	for i, orient := range p.Orientations {
		if cellsKey(orient) == key {
			return true, i
		}
	}
	return false, -1
}

// RenderCompact returns a compact ASCII representation of the piece's first orientation.
func (p *Piece) RenderCompact() string {
	if len(p.Orientations) == 0 {
		return ""
	}
	return RenderCells(p.Orientations[0])
}

// RenderCellOffsets renders cells as a coordinate list like "(0,0),(0,1),(1,0)".
func RenderCellOffsets(cells []Cell) string {
	parts := make([]string, len(cells))
	for i, c := range cells {
		parts[i] = fmt.Sprintf("(%d,%d)", c.Row, c.Col)
	}
	return strings.Join(parts, ",")
}

// RenderCells renders a set of normalized cells as a compact grid string.
// Rows are separated by /, X=filled, .=empty.
func RenderCells(cells []Cell) string {
	maxR, maxC := 0, 0
	for _, c := range cells {
		if c.Row > maxR {
			maxR = c.Row
		}
		if c.Col > maxC {
			maxC = c.Col
		}
	}
	grid := make([][]byte, maxR+1)
	for r := range grid {
		grid[r] = make([]byte, maxC+1)
		for c := range grid[r] {
			grid[r][c] = '.'
		}
	}
	for _, c := range cells {
		grid[c.Row][c.Col] = 'X'
	}
	rows := make([]string, len(grid))
	for i, row := range grid {
		rows[i] = string(row)
	}
	return strings.Join(rows, "/")
}
