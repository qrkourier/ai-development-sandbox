package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/netfoundry/workspace-agent/harness/internal/manager"
	"github.com/netfoundry/workspace-agent/harness/internal/mattermost"
	"github.com/netfoundry/workspace-agent/harness/internal/resources"
	"github.com/netfoundry/workspace-agent/harness/internal/state"
	"github.com/netfoundry/workspace-agent/harness/internal/tui"
	"github.com/netfoundry/workspace-agent/harness/internal/web"
)

func main() {
	var (
		configPath = flag.String("config", "/app/workspace.yaml", "Path to workspace.yaml")
		webPort    = flag.Int("web-port", 8090, "Web UI port")
		sshPort    = flag.Int("ssh-port", 2222, "SSH server port")
		noTUI      = flag.Bool("no-tui", false, "Disable TUI (run headless)")
	)
	flag.Parse()

	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		Prefix:          "harness",
	})

	// Load workspace config
	cfg, err := state.LoadConfig(*configPath)
	if err != nil {
		logger.Fatal("Failed to load config", "path", *configPath, "error", err)
	}

	// Initialize shared state
	appState := state.New(cfg)
	if err := appState.Recover(); err != nil {
		logger.Warn("State recovery failed (starting fresh)", "error", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("Shutting down...")
		cancel()
	}()

	// Start system resource monitor
	resMon := resources.NewMonitor(appState, logger)
	go resMon.Run(ctx)

	// Start Mattermost bridge
	mmBridge, err := mattermost.NewBridge(
		os.Getenv("MM_WS_URL"),
		os.Getenv("MM_API_URL"),
		os.Getenv("MM_BOT_TOKEN"),
		appState,
		logger,
	)
	if err != nil {
		logger.Warn("Mattermost bridge not configured", "error", err)
	} else {
		go mmBridge.Run(ctx)
	}

	// Start manager lifecycle
	mgr := manager.New(appState, mmBridge, logger)
	go mgr.Run(ctx)

	// Start web UI
	webServer := web.NewServer(*webPort, appState, logger)
	go webServer.Run(ctx)

	// Start TUI (blocks) or wait for signal
	if *noTUI {
		logger.Info("Running headless", "web_port", *webPort, "ssh_port", *sshPort)
		<-ctx.Done()
	} else {
		if err := tui.Run(appState, logger); err != nil {
			logger.Fatal("TUI error", "error", err)
		}
	}

	// Persist state on shutdown
	if err := appState.Save(); err != nil {
		logger.Error("Failed to save state", "error", err)
	}

	fmt.Println("Harness stopped.")
}
