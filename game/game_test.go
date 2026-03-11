package game

import (
	"testing"
)

func TestAllPiecesCount(t *testing.T) {
	pieces := AllPieces()
	if len(pieces) != 21 {
		t.Errorf("expected 21 pieces, got %d", len(pieces))
	}
}

func TestPieceSizes(t *testing.T) {
	pieces := AllPieces()
	sizeCounts := map[int]int{}
	for _, p := range pieces {
		sizeCounts[p.Size]++
		// Verify each orientation has the right number of cells
		for i, orient := range p.Orientations {
			if len(orient) != p.Size {
				t.Errorf("piece %s orientation %d has %d cells, expected %d",
					p.Name, i, len(orient), p.Size)
			}
		}
	}

	expected := map[int]int{1: 1, 2: 1, 3: 2, 4: 5, 5: 12}
	for size, count := range expected {
		if sizeCounts[size] != count {
			t.Errorf("expected %d pieces of size %d, got %d", count, size, sizeCounts[size])
		}
	}
}

func TestOrientationCounts(t *testing.T) {
	// Expected unique orientations for each piece
	expected := map[string]int{
		"O1": 1,  // point — all orientations identical
		"I2": 2,  // horizontal, vertical
		"I3": 2,  // horizontal, vertical
		"L3": 4,  // 4 rotations (flip = rotation)
		"I4": 2,  // horizontal, vertical
		"O4": 1,  // square — all orientations identical
		"T4": 4,  // 4 rotations (symmetric under flip)
		"S4": 4,  // 2 rotations × 2 flips (S ≠ Z)
		"L4": 8,  // asymmetric: 4 rotations × 2 flips
		"F5": 8,  // asymmetric
		"I5": 2,  // horizontal, vertical
		"L5": 8,  // asymmetric
		"N5": 8,  // asymmetric
		"P5": 8,  // asymmetric
		"T5": 4,  // symmetric under flip
		"U5": 4,  // symmetric under flip
		"V5": 4,  // symmetric under flip
		"W5": 4,  // symmetric under flip
		"X5": 1,  // plus sign — all orientations identical
		"Y5": 8,  // asymmetric
		"Z5": 4,  // 2 rotations × 2 flips (Z ≠ S)
	}

	pieces := PieceMap()
	for name, count := range expected {
		p := pieces[name]
		if p == nil {
			t.Errorf("piece %s not found", name)
			continue
		}
		if len(p.Orientations) != count {
			t.Errorf("piece %s: expected %d orientations, got %d",
				name, count, len(p.Orientations))
		}
	}
}

func TestNoPieceDuplicates(t *testing.T) {
	// Verify no two different pieces share an orientation
	pieces := AllPieces()
	seen := map[string]string{} // cellsKey -> piece name
	for _, p := range pieces {
		for _, orient := range p.Orientations {
			key := cellsKey(orient)
			if other, ok := seen[key]; ok {
				if other != p.Name {
					t.Errorf("pieces %s and %s share orientation %s", other, p.Name, key)
				}
			}
			seen[key] = p.Name
		}
	}
}

func TestFirstMoveMustCoverCorner(t *testing.T) {
	b := NewBoard(ClassicBoardSize)
	startCell := Cell{0, 0} // Player 1 corner

	// I3 not covering corner
	cells := []Cell{{0, 1}, {0, 2}, {0, 3}}
	err := b.ValidatePlacement(1, cells, true, startCell)
	if err == nil {
		t.Error("expected error for first move not covering corner")
	}

	// I3 covering corner
	cells = []Cell{{0, 0}, {0, 1}, {0, 2}}
	err = b.ValidatePlacement(1, cells, true, startCell)
	if err != nil {
		t.Errorf("expected valid first move, got: %v", err)
	}
}

func TestEdgeAdjacency(t *testing.T) {
	b := NewBoard(ClassicBoardSize)
	b.Grid[0][0] = 1 // Player 1 piece at (0,0)

	// Try placing adjacent (edge-sharing) — should fail
	cells := []Cell{{0, 1}} // right next to (0,0)
	err := b.ValidatePlacement(1, cells, false, Cell{0, 0})
	if err == nil {
		t.Error("expected error for edge-adjacent placement")
	}

	// Diagonal is OK
	cells = []Cell{{1, 1}}
	err = b.ValidatePlacement(1, cells, false, Cell{0, 0})
	if err != nil {
		t.Errorf("expected valid diagonal placement, got: %v", err)
	}
}

func TestMatchesOrientation(t *testing.T) {
	pieces := PieceMap()

	// I3 horizontal
	matched, _ := pieces["I3"].MatchesOrientation([]Cell{{5, 5}, {5, 6}, {5, 7}})
	if !matched {
		t.Error("expected I3 horizontal to match")
	}

	// I3 vertical
	matched, _ = pieces["I3"].MatchesOrientation([]Cell{{5, 5}, {6, 5}, {7, 5}})
	if !matched {
		t.Error("expected I3 vertical to match")
	}

	// Wrong shape for I3
	matched, _ = pieces["I3"].MatchesOrientation([]Cell{{5, 5}, {5, 6}, {6, 5}})
	if matched {
		t.Error("expected L-shape to NOT match I3")
	}
}

func TestDuoBoard(t *testing.T) {
	g := NewGame(ModeDuo, 2)
	if g.Board.Size != DuoBoardSize {
		t.Errorf("expected board size %d, got %d", DuoBoardSize, g.Board.Size)
	}
	if g.NumPlayers != 2 {
		t.Errorf("expected 2 players, got %d", g.NumPlayers)
	}

	// Check starting positions
	if g.Players[0].StartCell != (Cell{4, 4}) {
		t.Errorf("expected player 1 start at (4,4), got %v", g.Players[0].StartCell)
	}
	if g.Players[1].StartCell != (Cell{9, 9}) {
		t.Errorf("expected player 2 start at (9,9), got %v", g.Players[1].StartCell)
	}

	// First move must cover starting position
	cells := []Cell{{4, 4}, {4, 5}, {4, 6}}
	err := g.Board.ValidatePlacement(1, cells, true, g.Players[0].StartCell)
	if err != nil {
		t.Errorf("expected valid first move covering (4,4), got: %v", err)
	}

	// First move NOT covering starting position should fail
	cells = []Cell{{0, 0}, {0, 1}, {0, 2}}
	err = g.Board.ValidatePlacement(1, cells, true, g.Players[0].StartCell)
	if err == nil {
		t.Error("expected error for first move not covering starting position")
	}
}

func TestClassicBoard(t *testing.T) {
	g := NewGame(ModeClassic, 4)
	if g.Board.Size != ClassicBoardSize {
		t.Errorf("expected board size %d, got %d", ClassicBoardSize, g.Board.Size)
	}
	if g.NumPlayers != 4 {
		t.Errorf("expected 4 players, got %d", g.NumPlayers)
	}

	// Check corner starting positions
	if g.Players[0].StartCell != (Cell{0, 0}) {
		t.Errorf("expected player 1 start at (0,0), got %v", g.Players[0].StartCell)
	}
	if g.Players[1].StartCell != (Cell{0, 19}) {
		t.Errorf("expected player 2 start at (0,19), got %v", g.Players[1].StartCell)
	}
}
