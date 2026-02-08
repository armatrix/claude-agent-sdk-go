package checkpoint

import (
	"fmt"
	"os"
	"sync"
)

// FileChange records the original content of a file before modification.
type FileChange struct {
	Path            string
	OriginalContent []byte
	OriginalExists  bool
}

// Tracker records file changes for rewind capability.
// It captures the original content of files before they are modified,
// enabling rollback to a previous state.
type Tracker struct {
	mu      sync.RWMutex
	changes map[string]*FileChange
}

// NewTracker creates a new empty checkpoint tracker.
func NewTracker() *Tracker {
	return &Tracker{
		changes: make(map[string]*FileChange),
	}
}

// RecordWrite records the original content of a file before it is written.
// Only the first write to each path is recorded (preserving the true original).
func (t *Tracker) RecordWrite(path string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Only record the first change per path
	if _, exists := t.changes[path]; exists {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet — record that it was new
			t.changes[path] = &FileChange{
				Path:           path,
				OriginalExists: false,
			}
			return nil
		}
		return fmt.Errorf("checkpoint: cannot read %s: %w", path, err)
	}

	t.changes[path] = &FileChange{
		Path:            path,
		OriginalContent: data,
		OriginalExists:  true,
	}
	return nil
}

// Rewind restores all tracked files to their original state.
// Files that were newly created are deleted.
// Files that were modified are restored to their original content.
func (t *Tracker) Rewind() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var firstErr error
	for path, change := range t.changes {
		var err error
		if !change.OriginalExists {
			err = os.Remove(path)
			if os.IsNotExist(err) {
				err = nil // already gone
			}
		} else {
			err = os.WriteFile(path, change.OriginalContent, 0644)
		}
		if err != nil && firstErr == nil {
			firstErr = fmt.Errorf("checkpoint: rewind %s: %w", path, err)
		}
	}

	// Clear changes after rewind
	t.changes = make(map[string]*FileChange)
	return firstErr
}

// Changes returns the number of tracked file changes.
func (t *Tracker) Changes() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.changes)
}

// Paths returns the paths of all tracked file changes.
func (t *Tracker) Paths() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	paths := make([]string, 0, len(t.changes))
	for p := range t.changes {
		paths = append(paths, p)
	}
	return paths
}

// Clear discards all tracked changes without restoring files.
func (t *Tracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.changes = make(map[string]*FileChange)
}
