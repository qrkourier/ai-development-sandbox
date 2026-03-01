package hijack

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/netfoundry/workspace-agent/harness/internal/state"
)

// Monitor watches for director hijack of worker containers via tmux client count
type Monitor struct {
	state  *state.AppState
	logger *log.Logger
}

// NewMonitor creates a hijack monitor
func NewMonitor(appState *state.AppState, logger *log.Logger) *Monitor {
	return &Monitor{
		state:  appState,
		logger: logger,
	}
}

// Run polls tmux client counts for all active workers
func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAllWorkers()
		}
	}
}

func (m *Monitor) checkAllWorkers() {
	for _, w := range m.state.ListWorkers() {
		if w.Status != state.WorkerRunning && w.Status != state.WorkerHijacked {
			continue
		}
		clients, err := getTmuxClientCount(w.ContainerID)
		if err != nil {
			continue // tmux not running in this container, skip
		}
		if clients > 0 && w.Status == state.WorkerRunning {
			m.logger.Info("Director hijack detected", "worker", w.ID, "clients", clients)
			w.Status = state.WorkerHijacked
		} else if clients == 0 && w.Status == state.WorkerHijacked {
			m.logger.Info("Director detached from worker", "worker", w.ID)
			w.Status = state.WorkerRunning
		}
	}
}

// getTmuxClientCount checks how many tmux clients are attached in a container
func getTmuxClientCount(containerID string) (int, error) {
	cmd := exec.Command("docker", "exec", containerID, "tmux", "list-clients", "-F", "#{client_name}")
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("tmux list-clients: %w", err)
	}
	lines := strings.TrimSpace(string(out))
	if lines == "" {
		return 0, nil
	}
	return len(strings.Split(lines, "\n")), nil
}

// GetAttachCommand returns the command for the director to attach to a worker
func GetAttachCommand(containerID string) string {
	return fmt.Sprintf("docker exec -it %s tmux attach", containerID)
}

// CountTmuxClients is exported for use by manager
func CountTmuxClients(containerID string) (int, error) {
	count, err := strconv.Atoi(strings.TrimSpace("0"))
	if err != nil {
		return 0, err
	}
	_ = count
	return getTmuxClientCount(containerID)
}
