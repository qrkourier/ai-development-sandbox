package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/netfoundry/workspace-agent/harness/internal/state"
)

// Model is the main TUI dashboard model
type Model struct {
	state    *state.AppState
	logger   *log.Logger
	width    int
	height   int
	selected int
	quitting bool
}

func newModel(appState *state.AppState, logger *log.Logger) Model {
	return Model{
		state:  appState,
		logger: logger,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "j", "down":
			workers := m.state.ListWorkers()
			if m.selected < len(workers)-1 {
				m.selected++
			}
		case "k", "up":
			if m.selected > 0 {
				m.selected--
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return "Goodbye.\n"
	}

	var b strings.Builder

	// Header
	b.WriteString("  Workspace Agent Harness\n")
	b.WriteString("  ━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Traffic light
	tl := m.state.TrafficLightStatus()
	var tlIndicator string
	switch tl {
	case state.TrafficGreen:
		tlIndicator = "[GREEN]"
	case state.TrafficYellow:
		tlIndicator = "[YELLOW]"
	case state.TrafficRed:
		tlIndicator = "[RED]"
	}
	b.WriteString(fmt.Sprintf("  API Usage: %s\n\n", tlIndicator))

	// System resources
	snap := m.state.LatestResource()
	if snap != nil {
		b.WriteString(fmt.Sprintf("  CPU: %.1f%%  RAM: %dMB/%dMB  GPU: %.1f%%  Disk: %.1fGB/%.1fGB\n\n",
			snap.CPUPercent,
			snap.RAMUsedMB, snap.RAMTotalMB,
			snap.GPUPercent,
			snap.DiskUsedGB, snap.DiskTotalGB,
		))
	}

	// Workers table
	workers := m.state.ListWorkers()
	if len(workers) == 0 {
		b.WriteString("  No active workers.\n")
	} else {
		b.WriteString("  ID            Project    Type       Status     Tokens\n")
		b.WriteString("  ─────────────────────────────────────────────────────\n")
		for i, w := range workers {
			cursor := "  "
			if i == m.selected {
				cursor = "▸ "
			}
			id := w.ID
			if len(id) > 12 {
				id = id[:12]
			}
			b.WriteString(fmt.Sprintf("%s%-14s %-10s %-10s %-10s %d\n",
				cursor, id, w.Project, w.WorkerType, w.Status, w.TokenCount,
			))
		}
	}

	b.WriteString("\n  Manager PID: ")
	pid := m.state.ManagerPID()
	if pid > 0 {
		b.WriteString(fmt.Sprintf("%d", pid))
	} else {
		b.WriteString("not running")
	}

	b.WriteString("\n\n  q: quit  j/k: navigate\n")
	return b.String()
}

// Run starts the Bubble Tea TUI
func Run(appState *state.AppState, logger *log.Logger) error {
	m := newModel(appState, logger)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
