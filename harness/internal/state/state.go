package state

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Config represents workspace.yaml
type Config struct {
	Name     string    `yaml:"name" json:"name"`
	Projects []Project `yaml:"projects" json:"projects"`
	Privileges []Privilege `yaml:"privileges" json:"privileges"`
	Models   Models    `yaml:"models" json:"models"`
	LiteLLM  LiteLLM  `yaml:"litellm" json:"litellm"`
	Mattermost MattermostConfig `yaml:"mattermost" json:"mattermost"`
	Supervision Supervision `yaml:"supervision" json:"supervision"`
	Alerts   Alerts    `yaml:"alerts" json:"alerts"`
}

type Project struct {
	Alias     string `yaml:"alias" json:"alias"`
	Path      string `yaml:"path" json:"path"`
	Isolation string `yaml:"isolation" json:"isolation"` // worktree | bind | copy
}

type Privilege struct {
	ID          string `yaml:"id" json:"id"`
	Description string `yaml:"description" json:"description"`
	Grant       string `yaml:"grant" json:"grant"` // per-task | permanent
	TokenEnv    string `yaml:"token_env,omitempty" json:"token_env,omitempty"`
	Mount       string `yaml:"mount,omitempty" json:"mount,omitempty"`
	Mode        string `yaml:"mode,omitempty" json:"mode,omitempty"`
}

type Models struct {
	Preprocessing  string `yaml:"preprocessing" json:"preprocessing"`
	LocalWorker    string `yaml:"local_worker" json:"local_worker"`
	FrontierWorker string `yaml:"frontier_worker" json:"frontier_worker"`
	Embeddings     string `yaml:"embeddings" json:"embeddings"`
}

type LiteLLM struct {
	RoutingStrategy string   `yaml:"routing_strategy" json:"routing_strategy"`
	FallbackModels  []string `yaml:"fallback_models" json:"fallback_models"`
}

type MattermostConfig struct {
	Channel string `yaml:"channel" json:"channel"`
}

type Supervision struct {
	StuckTimeoutMinutes       int `yaml:"stuck_timeout_minutes" json:"stuck_timeout_minutes"`
	MaxSpawnRetries           int `yaml:"max_spawn_retries" json:"max_spawn_retries"`
	TokenBudgetPerTask        int `yaml:"token_budget_per_task" json:"token_budget_per_task"`
	ResourceSampleIntervalSec int `yaml:"resource_sample_interval_seconds" json:"resource_sample_interval_seconds"`
}

type Alerts struct {
	GPUUtilizationPercent    int `yaml:"gpu_utilization_percent" json:"gpu_utilization_percent"`
	GPUAlertDurationMinutes  int `yaml:"gpu_alert_duration_minutes" json:"gpu_alert_duration_minutes"`
	RAMAvailablePercentMin   int `yaml:"ram_available_percent_min" json:"ram_available_percent_min"`
	DiskFreeGBMin            int `yaml:"disk_free_gb_min" json:"disk_free_gb_min"`
}

// WorkerStatus represents the state of a running worker
type WorkerStatus string

const (
	WorkerRunning  WorkerStatus = "running"
	WorkerStuck    WorkerStatus = "stuck"
	WorkerHijacked WorkerStatus = "hijacked"
	WorkerDone     WorkerStatus = "completed"
	WorkerFailed   WorkerStatus = "failed"
)

// Worker tracks a running worker container
type Worker struct {
	ID           string       `json:"id"`
	ContainerID  string       `json:"container_id"`
	Project      string       `json:"project"`
	ThreadID     string       `json:"thread_id"`
	WorkerType   string       `json:"worker_type"` // frontier | local
	Status       WorkerStatus `json:"status"`
	SpawnedAt    time.Time    `json:"spawned_at"`
	LastOutput   time.Time    `json:"last_output"`
	TokenCount   int64        `json:"token_count"`
	SpawnCount   int          `json:"spawn_count"`
	WorktreePath string       `json:"worktree_path"`
	Privileges   []string     `json:"privileges"`
}

// TrafficLight represents API usage status
type TrafficLight string

const (
	TrafficGreen  TrafficLight = "green"
	TrafficYellow TrafficLight = "yellow"
	TrafficRed    TrafficLight = "red"
)

// ResourceSnapshot holds a point-in-time system resource reading
type ResourceSnapshot struct {
	Timestamp    time.Time `json:"timestamp"`
	CPUPercent   float64   `json:"cpu_percent"`
	RAMUsedMB    int64     `json:"ram_used_mb"`
	RAMTotalMB   int64     `json:"ram_total_mb"`
	GPUPercent   float64   `json:"gpu_percent"`
	VRAMUsedMB   int64     `json:"vram_used_mb"`
	VRAMTotalMB  int64     `json:"vram_total_mb"`
	DiskUsedGB   float64   `json:"disk_used_gb"`
	DiskTotalGB  float64   `json:"disk_total_gb"`
}

// AppState is the shared state for the harness
type AppState struct {
	mu        sync.RWMutex
	config    *Config
	workers   map[string]*Worker
	managerPID int
	trafficLight TrafficLight
	resources  []ResourceSnapshot // rolling 24h window
	statePath  string
}

// New creates a new AppState from config
func New(cfg *Config) *AppState {
	return &AppState{
		config:       cfg,
		workers:      make(map[string]*Worker),
		trafficLight: TrafficGreen,
		resources:    make([]ResourceSnapshot, 0, 1440), // 24h at 1-min intervals
		statePath:    "workspace-state.json",
	}
}

// Config returns the workspace config
func (s *AppState) Config() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// AddWorker registers a new worker
func (s *AppState) AddWorker(w *Worker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workers[w.ID] = w
}

// GetWorker returns a worker by ID
func (s *AppState) GetWorker(id string) *Worker {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.workers[id]
}

// ListWorkers returns all workers
func (s *AppState) ListWorkers() []*Worker {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Worker, 0, len(s.workers))
	for _, w := range s.workers {
		result = append(result, w)
	}
	return result
}

// RemoveWorker removes a worker by ID
func (s *AppState) RemoveWorker(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.workers, id)
}

// SetManagerPID records the manager process ID
func (s *AppState) SetManagerPID(pid int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.managerPID = pid
}

// ManagerPID returns the current manager PID
func (s *AppState) ManagerPID() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.managerPID
}

// SetTrafficLight updates the API usage status
func (s *AppState) SetTrafficLight(tl TrafficLight) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trafficLight = tl
}

// TrafficLightStatus returns the current traffic light
func (s *AppState) TrafficLightStatus() TrafficLight {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.trafficLight
}

// AddResourceSnapshot adds a resource reading
func (s *AppState) AddResourceSnapshot(snap ResourceSnapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources = append(s.resources, snap)
	// Trim to 24h window (1440 entries at 1-min intervals)
	if len(s.resources) > 1440 {
		s.resources = s.resources[len(s.resources)-1440:]
	}
}

// LatestResource returns the most recent resource snapshot
func (s *AppState) LatestResource() *ResourceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.resources) == 0 {
		return nil
	}
	snap := s.resources[len(s.resources)-1]
	return &snap
}

// persistedState is the JSON-serializable form of AppState
type persistedState struct {
	Workers      map[string]*Worker `json:"workers"`
	ManagerPID   int                `json:"manager_pid"`
	TrafficLight TrafficLight       `json:"traffic_light"`
}

// Save persists state to disk
func (s *AppState) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ps := persistedState{
		Workers:      s.workers,
		ManagerPID:   s.managerPID,
		TrafficLight: s.trafficLight,
	}
	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.statePath, data, 0644)
}

// Recover loads state from disk
func (s *AppState) Recover() error {
	data, err := os.ReadFile(s.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no state file, fresh start
		}
		return err
	}
	var ps persistedState
	if err := json.Unmarshal(data, &ps); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workers = ps.Workers
	s.managerPID = ps.ManagerPID
	s.trafficLight = ps.TrafficLight
	return nil
}
