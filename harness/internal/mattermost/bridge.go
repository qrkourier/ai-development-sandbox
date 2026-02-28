package mattermost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gorilla/websocket"
	"github.com/netfoundry/workspace-agent/harness/internal/state"
)

// Bridge manages the Mattermost WebSocket connection
type Bridge struct {
	wsURL    string
	apiURL   string
	botToken string
	state    *state.AppState
	logger   *log.Logger
	conn     *websocket.Conn
	mu       sync.Mutex
	msgCh    chan Message
}

// Message represents an incoming Mattermost message
type Message struct {
	ThreadID  string
	ChannelID string
	UserID    string
	Username  string
	Text      string
	PostID    string
}

// NewBridge creates a new Mattermost bridge
func NewBridge(wsURL, apiURL, botToken string, appState *state.AppState, logger *log.Logger) (*Bridge, error) {
	if wsURL == "" || botToken == "" {
		return nil, fmt.Errorf("MM_WS_URL and MM_BOT_TOKEN are required")
	}
	return &Bridge{
		wsURL:    wsURL,
		apiURL:   apiURL,
		botToken: botToken,
		state:    appState,
		logger:   logger,
		msgCh:    make(chan Message, 100),
	}, nil
}

// Messages returns the channel of incoming messages
func (b *Bridge) Messages() <-chan Message {
	return b.msgCh
}

// Run connects to Mattermost WebSocket and processes events
func (b *Bridge) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := b.connect(ctx); err != nil {
				b.logger.Error("Mattermost connection failed", "error", err)
			}
			// Exponential backoff on reconnect
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				b.logger.Info("Reconnecting to Mattermost...")
			}
		}
	}
}

func (b *Bridge) connect(ctx context.Context) error {
	wsURL := b.wsURL + "/api/v4/websocket"
	header := http.Header{}
	header.Set("Authorization", "Bearer "+b.botToken)

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	b.mu.Lock()
	b.conn = conn
	b.mu.Unlock()
	defer conn.Close()

	// Authenticate
	authMsg := map[string]interface{}{
		"seq":    1,
		"action": "authentication_challenge",
		"data":   map[string]string{"token": b.botToken},
	}
	if err := conn.WriteJSON(authMsg); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	b.logger.Info("Connected to Mattermost WebSocket")

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			_, rawMsg, err := conn.ReadMessage()
			if err != nil {
				return fmt.Errorf("read: %w", err)
			}
			b.handleEvent(rawMsg)
		}
	}
}

func (b *Bridge) handleEvent(raw []byte) {
	var event struct {
		Event string                 `json:"event"`
		Data  map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return
	}

	if event.Event != "posted" {
		return
	}

	postJSON, ok := event.Data["post"].(string)
	if !ok {
		return
	}

	var post struct {
		ID        string `json:"id"`
		ChannelID string `json:"channel_id"`
		UserID    string `json:"user_id"`
		Message   string `json:"message"`
		RootID    string `json:"root_id"`
	}
	if err := json.Unmarshal([]byte(postJSON), &post); err != nil {
		return
	}

	threadID := post.RootID
	if threadID == "" {
		threadID = post.ID
	}

	msg := Message{
		ThreadID:  threadID,
		ChannelID: post.ChannelID,
		UserID:    post.UserID,
		Text:      post.Message,
		PostID:    post.ID,
	}

	select {
	case b.msgCh <- msg:
	default:
		b.logger.Warn("Message channel full, dropping message")
	}
}

// PostMessage sends a message to a Mattermost channel/thread
func (b *Bridge) PostMessage(channelID, rootID, message string) error {
	url := b.apiURL + "/api/v4/posts"
	body := map[string]string{
		"channel_id": channelID,
		"message":    message,
	}
	if rootID != "" {
		body["root_id"] = rootID
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", url, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+b.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("mattermost API returned %d", resp.StatusCode)
	}
	return nil
}
