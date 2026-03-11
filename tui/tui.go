package tui

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/perbu/ai-blokus/debug"
	"github.com/perbu/ai-blokus/game"
	"github.com/perbu/ai-blokus/player"
)

var (
	playerColors = map[int]color.Color{
		1: lipgloss.Color("196"), // Red
		2: lipgloss.Color("33"),  // Blue
		3: lipgloss.Color("226"), // Yellow
		4: lipgloss.Color("34"),  // Green
	}

	boardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2)

	infoStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2).
			Width(44)

	logStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242"))
)

// Model is the Bubble Tea model for the Blokus TUI.
type Model struct {
	game      *game.Game
	players   []player.Player
	ctx       context.Context
	cancel    context.CancelFunc
	err       error
	waiting    bool         // waiting for LLM response
	tokenCount *atomic.Int64 // number of tokens received so far (pointer to survive value copies)
	status     string       // current status message
	turnStart  time.Time    // when the current turn started
}

type moveResultMsg struct {
	move *player.Move
	err  error
}

type tickMsg time.Time

const turnTimeout = 60 * time.Second

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// New creates a new TUI model.
func New(g *game.Game, players []player.Player) Model {
	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		game:       g,
		players:    players,
		ctx:        ctx,
		cancel:     cancel,
		turnStart:  time.Now(),
		tokenCount: &atomic.Int64{},
	}
}

func (m Model) Init() tea.Cmd {
	return m.nextTurn()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.cancel()
			return m, tea.Quit
		}

	case tickMsg:
		if m.waiting {
			cur := m.game.CurrentPlayer()
			elapsed := time.Since(m.turnStart).Truncate(100 * time.Millisecond)
			tokens := m.tokenCount.Load()
			if tokens > 0 {
				m.status = fmt.Sprintf("Turn %d: %s receiving (%d tokens, %.1fs)",
					m.game.TurnNumber+1, cur.Color, tokens, elapsed.Seconds())
			} else {
				m.status = fmt.Sprintf("Turn %d: %s waiting... (%.1fs)",
					m.game.TurnNumber+1, cur.Color, elapsed.Seconds())
			}

			if time.Since(m.turnStart) >= turnTimeout {
				debug.Log("[game] %s timed out after %v", cur.Color, turnTimeout)
				m.cancel()
				// Create a new context for subsequent turns
				m.ctx, m.cancel = context.WithCancel(context.Background())
				m.waiting = false
				durStr := fmt.Sprintf("%.1fs", elapsed.Seconds())
				m.status = fmt.Sprintf("%s: timed out (%s)", cur.Color, durStr)
				m.game.Log = append(m.game.Log, game.LogEntry{
					PlayerID: cur.ID,
					Color:    cur.Color,
					IsPass:   true,
					Error:    "timed out",
					Comment:  fmt.Sprintf("No response within %v (%s)", turnTimeout, durStr),
				})
				m.game.AdvanceTurn()
				if m.game.GameOver {
					m.status = "Game over!"
					return m, nil
				}
				return m, m.nextTurn()
			}

			return m, tickCmd()
		}
		return m, nil

	case moveResultMsg:
		elapsed := time.Since(m.turnStart).Truncate(100 * time.Millisecond)
		m.waiting = false
		m.processMove(msg, elapsed)

		if m.game.GameOver {
			m.status = "Game over!"
			return m, nil
		}

		return m, m.nextTurn()
	}

	return m, nil
}

func (m *Model) processMove(msg moveResultMsg, elapsed time.Duration) {
	p := m.game.CurrentPlayer()
	durStr := fmt.Sprintf("%.1fs", elapsed.Seconds())

	if msg.err != nil {
		m.status = fmt.Sprintf("%s: move failed, skipping turn (%s)", p.Color, durStr)
		debug.Log("[game] %s move failed: %v", p.Color, msg.err)
		m.game.Log = append(m.game.Log, game.LogEntry{
			PlayerID: p.ID,
			Color:    p.Color,
			IsPass:   true,
			Error:    msg.err.Error(),
			Comment:  fmt.Sprintf("failed after retries (%s)", durStr),
		})
		m.game.AdvanceTurn()
		return
	}

	move := msg.move

	if move.Pass {
		debug.Log("[game] %s chose to pass: %s", p.Color, move.Comment)
		m.status = fmt.Sprintf("%s passed (%s)", p.Color, durStr)
		m.game.Log = append(m.game.Log, game.LogEntry{
			PlayerID: p.ID,
			Color:    p.Color,
			IsPass:   true,
			Comment:  fmt.Sprintf("%s (%s)", move.Comment, durStr),
		})
		m.game.AdvanceTurn()
		return
	}

	cells := move.ToCells()
	debug.Log("[game] %s attempting piece=%s cells=%v", p.Color, move.Piece, cells)
	err := m.game.ApplyMove(move.Piece, cells)
	if err != nil {
		debug.Log("[game] %s invalid move: %v", p.Color, err)
		m.status = fmt.Sprintf("%s: invalid move (%s), skipping (%s)", p.Color, err, durStr)
		m.game.Log = append(m.game.Log, game.LogEntry{
			PlayerID: p.ID,
			Color:    p.Color,
			Piece:    move.Piece,
			IsPass:   true,
			Error:    err.Error(),
			Comment:  fmt.Sprintf("%s (%s)", move.Comment, durStr),
		})
		m.game.AdvanceTurn()
		return
	}

	debug.Log("[game] %s placed %s at %v (%s)", p.Color, move.Piece, cells, durStr)
	m.status = fmt.Sprintf("%s placed %s (%s)", p.Color, move.Piece, durStr)
	m.game.Log = append(m.game.Log, game.LogEntry{
		PlayerID: p.ID,
		Color:    p.Color,
		Piece:    move.Piece,
		Comment:  fmt.Sprintf("%s (%s)", move.Comment, durStr),
	})
	m.game.AdvanceTurn()
}

// nextTurn finds the next player who can move and requests their move.
// Skips eliminated players. Returns nil cmd if game is over.
func (m *Model) nextTurn() tea.Cmd {
	g := m.game

	// Skip eliminated players, and check newly-current players for valid moves
	for i := 0; i < g.NumPlayers; i++ {
		if g.GameOver {
			m.status = "Game over!"
			return nil
		}

		cur := g.CurrentPlayer()
		if cur.Eliminated {
			g.AdvanceTurn()
			continue
		}

		if !g.CheckPlayerCanMove() {
			debug.Log("[game] %s eliminated (no valid moves)", cur.Color)
			g.Log = append(g.Log, game.LogEntry{
				PlayerID: cur.ID,
				Color:    cur.Color,
				IsPass:   true,
				Comment:  "No valid moves — eliminated",
			})
			g.AdvanceTurn()
			continue
		}

		// This player can move — request from LLM
		p := m.players[g.CurrentTurn]
		m.waiting = true
		m.tokenCount.Store(0)
		m.turnStart = time.Now()
		m.status = fmt.Sprintf("Turn %d: %s waiting... (0.0s)", g.TurnNumber+1, cur.Color)

		// Wire up the token callback if this is an LLM player
		if llm, ok := p.(*player.LLMPlayer); ok {
			counter := m.tokenCount
			llm.OnToken = func() {
				counter.Add(1)
			}
		}

		return tea.Batch(
			func() tea.Msg {
				move, err := p.GetMove(m.ctx, g)
				return moveResultMsg{move: move, err: err}
			},
			tickCmd(),
		)
	}

	// All players checked, none can move
	g.GameOver = true
	m.status = "Game over!"
	return nil
}

func (m Model) View() tea.View {
	board := m.renderBoard()
	info := m.renderInfo()

	top := lipgloss.JoinHorizontal(lipgloss.Top, board, "  ", info)
	topWidth := lipgloss.Width(top)

	log := m.renderLog(topWidth)

	statusStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))
	statusLine := statusStyle.Render(m.status)

	content := lipgloss.JoinVertical(lipgloss.Left, top, "", statusLine, "", log, "", dimStyle.Render("Press q to quit"))
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m Model) renderBoard() string {
	var sb strings.Builder

	title := "  BLOKUS"
	if m.game.Mode == game.ModeDuo {
		title = "  BLOKUS DUO"
	}
	sb.WriteString(titleStyle.Render(title))
	sb.WriteString("\n")

	boardSize := m.game.Board.Size

	// Column headers
	sb.WriteString("   ")
	for c := 0; c < boardSize; c++ {
		fmt.Fprintf(&sb, "%2d", c%10)
	}
	sb.WriteString("\n")

	for r := 0; r < boardSize; r++ {
		fmt.Fprintf(&sb, "%2d ", r)
		for c := 0; c < boardSize; c++ {
			cell := m.game.Board.Grid[r][c]
			if cell == 0 {
				sb.WriteString(dimStyle.Render("··"))
			} else {
				style := lipgloss.NewStyle().
					Foreground(playerColors[cell])
				sb.WriteString(style.Render("██"))
			}
		}
		sb.WriteString("\n")
	}

	return boardStyle.Render(sb.String())
}

func (m Model) renderInfo() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Players"))
	sb.WriteString("\n\n")

	for i, p := range m.game.Players {
		color := playerColors[p.ID]
		nameStyle := lipgloss.NewStyle().Foreground(color).Bold(true)

		marker := "  "
		if m.game.CurrentTurn == p.ID-1 && !m.game.GameOver {
			marker = "> "
		}

		fmt.Fprintf(&sb, "%s%s", marker, nameStyle.Render(p.Color))
		if p.Eliminated {
			sb.WriteString(dimStyle.Render(" [out]"))
		}
		sb.WriteString("\n")

		fmt.Fprintf(&sb, "    %s\n", dimStyle.Render(m.players[i].Model()))

		if m.waiting && m.game.CurrentTurn == p.ID-1 && !m.game.GameOver {
			elapsed := time.Since(m.turnStart).Truncate(100 * time.Millisecond)
			tokens := m.tokenCount.Load()
			stateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("229"))
			if tokens > 0 {
				fmt.Fprintf(&sb, "    %s\n", stateStyle.Render(
					fmt.Sprintf("receiving (%d tokens, %.1fs)", tokens, elapsed.Seconds())))
			} else {
				fmt.Fprintf(&sb, "    %s\n", stateStyle.Render(
					fmt.Sprintf("waiting... (%.1fs)", elapsed.Seconds())))
			}
		}

		fmt.Fprintf(&sb, "    Pieces: %d  Cells left: %d\n",
			len(p.Remaining), p.CellsRemaining)

		names := p.RemainingPieceNames()
		if len(names) > 0 {
			sb.WriteString("    ")
			sb.WriteString(dimStyle.Render(strings.Join(names, " ")))
		}
		sb.WriteString("\n\n")
	}

	if m.game.GameOver {
		sb.WriteString(titleStyle.Render("GAME OVER"))
		sb.WriteString("\n\n")
		for _, p := range m.game.Players {
			color := playerColors[p.ID]
			nameStyle := lipgloss.NewStyle().Foreground(color)
			fmt.Fprintf(&sb, "  %s: %d points\n", nameStyle.Render(p.Color), p.Score())
		}
	}

	return infoStyle.Render(sb.String())
}

const logBoxHeight = 12 // fixed outer height including border

func (m Model) renderLog(width int) string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Game Log"))
	sb.WriteString("\n")

	// Render all entries, then let the fixed-height box clip from the top
	for _, e := range m.game.Log {
		color := playerColors[e.PlayerID]
		nameStyle := lipgloss.NewStyle().Foreground(color).Bold(true)

		if e.Error != "" {
			fmt.Fprintf(&sb, "  %s: INVALID (%s)", nameStyle.Render(e.Color), e.Error)
		} else if e.IsPass {
			fmt.Fprintf(&sb, "  %s: passed", nameStyle.Render(e.Color))
		} else {
			fmt.Fprintf(&sb, "  %s: placed %s", nameStyle.Render(e.Color), e.Piece)
		}

		if e.Comment != "" {
			// Truncate long comments to prevent wrapping
			maxComment := width - 30
			if maxComment < 20 {
				maxComment = 20
			}
			comment := e.Comment
			if len(comment) > maxComment {
				comment = comment[:maxComment-1] + "…"
			}
			sb.WriteString(dimStyle.Render(fmt.Sprintf(" — %s", comment)))
		}
		sb.WriteString("\n")
	}

	content := sb.String()

	// Keep only the last N lines that fit in the box
	// Inner height = box height - border (2) - padding (0)
	innerHeight := logBoxHeight - 2
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) > innerHeight {
		lines = lines[len(lines)-innerHeight:]
	}
	// Pad to fixed height
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}
	content = strings.Join(lines, "\n")

	return logStyle.Width(width).Height(innerHeight).Render(content)
}
