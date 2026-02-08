package session_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/session"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "sessions")
}

func TestFileStore_NewCreatesDir(t *testing.T) {
	dir := tempDir(t)
	store, err := session.NewFileStore(dir)
	require.NoError(t, err)
	require.NotNil(t, store)

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestFileStore_SaveAndLoad(t *testing.T) {
	store, err := session.NewFileStore(tempDir(t))
	require.NoError(t, err)
	ctx := context.Background()

	s := makeSession("file-1")
	require.NoError(t, store.Save(ctx, s))

	loaded, err := store.Load(ctx, "file-1")
	require.NoError(t, err)
	assert.Equal(t, "file-1", loaded.ID)
	assert.Equal(t, anthropic.Model("claude-opus-4-6"), loaded.Metadata.Model)
	assert.True(t, loaded.Metadata.TotalCost.Equal(decimal.NewFromFloat(0.01)))
	assert.Equal(t, 1, loaded.Metadata.NumTurns)
	assert.Len(t, loaded.Messages, 1)
}

func TestFileStore_SaveNil(t *testing.T) {
	store, err := session.NewFileStore(tempDir(t))
	require.NoError(t, err)
	err = store.Save(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session is nil")
}

func TestFileStore_LoadNotFound(t *testing.T) {
	store, err := session.NewFileStore(tempDir(t))
	require.NoError(t, err)

	_, loadErr := store.Load(context.Background(), "nonexistent")
	require.Error(t, loadErr)
	assert.Contains(t, loadErr.Error(), "session not found")
}

func TestFileStore_Delete(t *testing.T) {
	store, err := session.NewFileStore(tempDir(t))
	require.NoError(t, err)
	ctx := context.Background()

	s := makeSession("file-del")
	require.NoError(t, store.Save(ctx, s))

	require.NoError(t, store.Delete(ctx, "file-del"))

	_, loadErr := store.Load(ctx, "file-del")
	require.Error(t, loadErr)
	assert.Contains(t, loadErr.Error(), "session not found")
}

func TestFileStore_DeleteNotFound(t *testing.T) {
	store, err := session.NewFileStore(tempDir(t))
	require.NoError(t, err)

	err = store.Delete(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestFileStore_List(t *testing.T) {
	store, err := session.NewFileStore(tempDir(t))
	require.NoError(t, err)
	ctx := context.Background()

	// Empty
	list, err := store.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)

	// Add sessions
	require.NoError(t, store.Save(ctx, makeSession("fa")))
	require.NoError(t, store.Save(ctx, makeSession("fb")))

	list, err = store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)

	ids := make(map[string]bool)
	for _, s := range list {
		ids[s.ID] = true
	}
	assert.True(t, ids["fa"])
	assert.True(t, ids["fb"])
}

func TestFileStore_Fork(t *testing.T) {
	store, err := session.NewFileStore(tempDir(t))
	require.NoError(t, err)
	ctx := context.Background()

	original := makeSession("fork-src")
	require.NoError(t, store.Save(ctx, original))

	forked, err := store.Fork(ctx, "fork-src")
	require.NoError(t, err)
	assert.NotEqual(t, "fork-src", forked.ID)
	assert.Len(t, forked.Messages, 1)

	// Both exist on disk
	list, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestFileStore_ForkNotFound(t *testing.T) {
	store, err := session.NewFileStore(tempDir(t))
	require.NoError(t, err)

	_, forkErr := store.Fork(context.Background(), "nonexistent")
	require.Error(t, forkErr)
	assert.Contains(t, forkErr.Error(), "session not found")
}

func TestFileStore_SaveOverwrite(t *testing.T) {
	store, err := session.NewFileStore(tempDir(t))
	require.NoError(t, err)
	ctx := context.Background()

	s1 := makeSession("overwrite-file")
	s1.Metadata.NumTurns = 1
	require.NoError(t, store.Save(ctx, s1))

	s2 := makeSession("overwrite-file")
	s2.Metadata.NumTurns = 10
	require.NoError(t, store.Save(ctx, s2))

	loaded, err := store.Load(ctx, "overwrite-file")
	require.NoError(t, err)
	assert.Equal(t, 10, loaded.Metadata.NumTurns)
}

func TestFileStore_RoundTripMetadata(t *testing.T) {
	store, err := session.NewFileStore(tempDir(t))
	require.NoError(t, err)
	ctx := context.Background()

	s := makeSession("metadata-rt")
	s.Metadata.TotalCost = decimal.RequireFromString("1.23456789")
	s.Metadata.TotalTokens = agent.Usage{
		InputTokens:  1000,
		OutputTokens: 500,
		CacheReadInputTokens: 200,
	}
	require.NoError(t, store.Save(ctx, s))

	loaded, err := store.Load(ctx, "metadata-rt")
	require.NoError(t, err)
	assert.True(t, s.Metadata.TotalCost.Equal(loaded.Metadata.TotalCost))
	assert.Equal(t, int64(1000), loaded.Metadata.TotalTokens.InputTokens)
	assert.Equal(t, int64(500), loaded.Metadata.TotalTokens.OutputTokens)
	assert.Equal(t, int64(200), loaded.Metadata.TotalTokens.CacheReadInputTokens)
}

func TestFileStore_ListSkipsNonJSON(t *testing.T) {
	dir := tempDir(t)
	store, err := session.NewFileStore(dir)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, store.Save(ctx, makeSession("valid")))

	// Create a non-JSON file in the directory
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not json"), 0o644))

	list, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "valid", list[0].ID)
}
