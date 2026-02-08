package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// FileStore persists sessions as individual JSON files in a directory.
// Each session is stored as {id}.json.
type FileStore struct {
	dir string
}

var _ agent.FullSessionStore = (*FileStore)(nil)

// NewFileStore creates a FileStore that saves sessions to the given directory.
// The directory is created if it does not exist.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	return &FileStore{dir: dir}, nil
}

// sessionJSON is the on-disk representation of a session.
type sessionJSON struct {
	ID        string                     `json:"id"`
	Messages  []anthropic.MessageParam   `json:"messages"`
	Metadata  metadataJSON               `json:"metadata"`
	CreatedAt time.Time                  `json:"created_at"`
	UpdatedAt time.Time                  `json:"updated_at"`
}

type metadataJSON struct {
	Model       string `json:"model"`
	TotalCost   string `json:"total_cost"`
	TotalTokens agent.Usage `json:"total_tokens"`
	NumTurns    int    `json:"num_turns"`
}

// Save writes a session to disk as JSON.
func (f *FileStore) Save(_ context.Context, session *agent.Session) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}

	data := sessionJSON{
		ID:       session.ID,
		Messages: session.Messages,
		Metadata: metadataJSON{
			Model:       string(session.Metadata.Model),
			TotalCost:   session.Metadata.TotalCost.String(),
			TotalTokens: session.Metadata.TotalTokens,
			NumTurns:    session.Metadata.NumTurns,
		},
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	path := f.path(session.ID)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}
	return nil
}

// Load reads a session from disk by ID.
func (f *FileStore) Load(_ context.Context, id string) (*agent.Session, error) {
	path := f.path(id)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found: %s", id)
		}
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var data sessionJSON
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	cost, err := decimal.NewFromString(data.Metadata.TotalCost)
	if err != nil {
		cost = decimal.Zero
	}

	return &agent.Session{
		ID:       data.ID,
		Messages: data.Messages,
		Metadata: agent.SessionMeta{
			Model:       anthropic.Model(data.Metadata.Model),
			TotalCost:   cost,
			TotalTokens: data.Metadata.TotalTokens,
			NumTurns:    data.Metadata.NumTurns,
		},
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}, nil
}

// Delete removes a session file from disk.
func (f *FileStore) Delete(_ context.Context, id string) error {
	path := f.path(id)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session not found: %s", id)
		}
		return fmt.Errorf("remove session file: %w", err)
	}
	return nil
}

// List returns all sessions stored on disk.
func (f *FileStore) List(_ context.Context) ([]*agent.Session, error) {
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		return nil, fmt.Errorf("read session dir: %w", err)
	}

	var sessions []*agent.Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		s, err := f.Load(context.Background(), id)
		if err != nil {
			continue // skip corrupt files
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

// Fork loads a session, clones it with a new ID, saves the clone, and returns it.
func (f *FileStore) Fork(ctx context.Context, id string) (*agent.Session, error) {
	original, err := f.Load(ctx, id)
	if err != nil {
		return nil, err
	}

	forked := original.Clone()
	if err := f.Save(ctx, forked); err != nil {
		return nil, fmt.Errorf("save forked session: %w", err)
	}
	return forked, nil
}

func (f *FileStore) path(id string) string {
	return filepath.Join(f.dir, id+".json")
}
