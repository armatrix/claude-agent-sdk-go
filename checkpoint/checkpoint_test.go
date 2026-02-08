package checkpoint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracker_RecordAndRewind_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	// Create original file
	require.NoError(t, os.WriteFile(path, []byte("original"), 0644))

	tracker := NewTracker()
	require.NoError(t, tracker.RecordWrite(path))
	assert.Equal(t, 1, tracker.Changes())

	// Modify the file
	require.NoError(t, os.WriteFile(path, []byte("modified"), 0644))

	// Rewind should restore original
	require.NoError(t, tracker.Rewind())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "original", string(data))
	assert.Equal(t, 0, tracker.Changes())
}

func TestTracker_RecordAndRewind_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	tracker := NewTracker()
	require.NoError(t, tracker.RecordWrite(path))

	// Create the file
	require.NoError(t, os.WriteFile(path, []byte("new content"), 0644))

	// Rewind should delete the file
	require.NoError(t, tracker.Rewind())

	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}

func TestTracker_OnlyRecordsFirstWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	require.NoError(t, os.WriteFile(path, []byte("v1"), 0644))

	tracker := NewTracker()
	require.NoError(t, tracker.RecordWrite(path))

	// Modify and record again
	require.NoError(t, os.WriteFile(path, []byte("v2"), 0644))
	require.NoError(t, tracker.RecordWrite(path))

	// Should still restore to v1
	require.NoError(t, os.WriteFile(path, []byte("v3"), 0644))
	require.NoError(t, tracker.Rewind())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "v1", string(data))
}

func TestTracker_Paths(t *testing.T) {
	tracker := NewTracker()
	dir := t.TempDir()

	p1 := filepath.Join(dir, "a.txt")
	p2 := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(p1, []byte("a"), 0644))
	require.NoError(t, os.WriteFile(p2, []byte("b"), 0644))

	require.NoError(t, tracker.RecordWrite(p1))
	require.NoError(t, tracker.RecordWrite(p2))

	paths := tracker.Paths()
	assert.Len(t, paths, 2)
	assert.Contains(t, paths, p1)
	assert.Contains(t, paths, p2)
}

func TestTracker_Clear(t *testing.T) {
	tracker := NewTracker()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("data"), 0644))

	require.NoError(t, tracker.RecordWrite(path))
	assert.Equal(t, 1, tracker.Changes())

	tracker.Clear()
	assert.Equal(t, 0, tracker.Changes())
}
