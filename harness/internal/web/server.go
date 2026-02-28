package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/gorilla/websocket"
	"github.com/netfoundry/workspace-agent/harness/internal/state"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Server is the web UI HTTP server
type Server struct {
	port   int
	state  *state.AppState
	logger *log.Logger
	tmpl   *template.Template
	wsClients map[*websocket.Conn]bool
	wsMu      sync.Mutex
}

// NewServer creates a new web server
func NewServer(port int, appState *state.AppState, logger *log.Logger) *Server {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		logger.Warn("Failed to parse templates (will use fallback)", "error", err)
	}
	return &Server{
		port:      port,
		state:     appState,
		logger:    logger,
		tmpl:      tmpl,
		wsClients: make(map[*websocket.Conn]bool),
	}
}

// Run starts the HTTP server
func (s *Server) Run(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/api/status", s.handleAPIStatus)
	mux.HandleFunc("/api/workers", s.handleAPIWorkers)
	mux.HandleFunc("/api/resources", s.handleAPIResources)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.Handle("/static/", http.FileServer(http.FS(staticFS)))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		srv.Close()
	}()

	s.logger.Info("Web UI listening", "port", s.port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		s.logger.Error("Web server error", "error", err)
	}
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if s.tmpl == nil {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<html><body><h1>Workspace Agent Dashboard</h1><p>Templates not loaded.</p></body></html>")
		return
	}
	data := map[string]interface{}{
		"Workers":      s.state.ListWorkers(),
		"TrafficLight": s.state.TrafficLightStatus(),
		"Resources":    s.state.LatestResource(),
		"ManagerPID":   s.state.ManagerPID(),
	}
	if err := s.tmpl.ExecuteTemplate(w, "dashboard.html", data); err != nil {
		s.logger.Error("Template error", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"traffic_light": s.state.TrafficLightStatus(),
		"manager_pid":   s.state.ManagerPID(),
		"worker_count":  len(s.state.ListWorkers()),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (s *Server) handleAPIWorkers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.state.ListWorkers())
}

func (s *Server) handleAPIResources(w http.ResponseWriter, r *http.Request) {
	snap := s.state.LatestResource()
	w.Header().Set("Content-Type", "application/json")
	if snap != nil {
		json.NewEncoder(w).Encode(snap)
	} else {
		json.NewEncoder(w).Encode(map[string]string{"status": "no data"})
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed", "error", err)
		return
	}
	s.wsMu.Lock()
	s.wsClients[conn] = true
	s.wsMu.Unlock()

	defer func() {
		s.wsMu.Lock()
		delete(s.wsClients, conn)
		s.wsMu.Unlock()
		conn.Close()
	}()

	// Read messages (keep connection alive)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

// Broadcast sends a message to all connected WebSocket clients
func (s *Server) Broadcast(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	for conn := range s.wsClients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			conn.Close()
			delete(s.wsClients, conn)
		}
	}
}
