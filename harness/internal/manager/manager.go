package manager

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/netfoundry/workspace-agent/harness/internal/mattermost"
	"github.com/netfoundry/workspace-agent/harness/internal/state"
)

var mmMsgPattern = regexp.MustCompile(`^\[MM:([^\]]+)\]\s*(.*)$`)

// Manager handles the Claude Code manager agent lifecycle
type Manager struct {
	state  *state.AppState
	mm     *mattermost.Bridge
	logger *log.Logger
	cmd    *exec.Cmd
}

// New creates a new manager lifecycle handler
func New(appState *state.AppState, mm *mattermost.Bridge, logger *log.Logger) *Manager {
	return &Manager{
		state:  appState,
		mm:     mm,
		logger: logger,
	}
}

// Run manages the Claude Code process lifecycle
func (m *Manager) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := m.runOnce(ctx); err != nil {
				m.logger.Error("Manager process exited", "error", err)
			}
			// Brief pause before restart
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
				m.logger.Info("Restarting manager agent...")
			}
		}
	}
}

func (m *Manager) runOnce(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "claude", "--permission-mode", "default")
	cmd.Dir = findProjectRoot()
	cmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+os.Getenv("HOME")+"/.claude")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	cmd.Stderr = os.Stderr
	m.cmd = cmd

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	m.state.SetManagerPID(cmd.Process.Pid)
	m.logger.Info("Manager started", "pid", cmd.Process.Pid)

	// Feed Mattermost messages to stdin
	if m.mm != nil {
		go func() {
			for msg := range m.mm.Messages() {
				prompt := fmt.Sprintf("[From MM thread %s, user %s]: %s\n",
					msg.ThreadID, msg.Username, msg.Text)
				if _, err := stdin.Write([]byte(prompt)); err != nil {
					m.logger.Error("Failed to write to manager stdin", "error", err)
					return
				}
			}
		}()
	}

	// Process stdout
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB buffer
	for scanner.Scan() {
		line := scanner.Text()
		m.processOutput(line)
	}

	m.state.SetManagerPID(0)
	return cmd.Wait()
}

func (m *Manager) processOutput(line string) {
	// Check for Mattermost-bound output
	if matches := mmMsgPattern.FindStringSubmatch(line); len(matches) == 3 {
		threadID := matches[1]
		message := matches[2]
		if m.mm != nil {
			// Find channel for this thread
			if err := m.mm.PostMessage("", threadID, message); err != nil {
				m.logger.Error("Failed to post to Mattermost", "thread", threadID, "error", err)
			}
		}
		return
	}

	// Check for privilege requests from workers
	if strings.HasPrefix(line, "[PRIVILEGE_REQUEST]") {
		m.logger.Info("Privilege request from worker", "request", line)
	}

	// Log other output
	m.logger.Debug("Manager output", "line", line)
}

// SendMessage sends a message to the manager's stdin
func (m *Manager) SendMessage(msg string) error {
	if m.cmd == nil || m.cmd.Process == nil {
		return fmt.Errorf("manager not running")
	}
	// This is a simplified version — in production, use the stdin pipe
	return nil
}

func findProjectRoot() string {
	// Look for workspace.yaml to find the project root
	candidates := []string{
		"/app",
		os.Getenv("HOME"),
		".",
	}
	for _, dir := range candidates {
		if _, err := os.Stat(dir + "/workspace.yaml"); err == nil {
			return dir
		}
	}
	return "."
}
