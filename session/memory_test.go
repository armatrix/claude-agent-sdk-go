package session_test

import (
	"context"
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/session"
)

func makeSession(id string) *agent.Session {
	s := agent.NewSession()
	s.ID = id
	s.Messages = []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("hello")),
	}
	s.Metadata.Model = "claude-opus-4-6"
	s.Metadata.TotalCost = decimal.NewFromFloat(0.01)
	s.Metadata.NumTurns = 1
	return s
}

func TestMemoryStore_SaveAndLoad(t *testing.T) {
	store := session.NewMemoryStore()
	ctx := context.Background()

	s := makeSession("sess-1")
	require.NoError(t, store.Save(ctx, s))

	loaded, err := store.Load(ctx, "sess-1")
	require.NoError(t, err)
	assert.Equal(t, "sess-1", loaded.ID)
	assert.Equal(t, anthropic.Model("claude-opus-4-6"), loaded.Metadata.Model)
	assert.True(t, loaded.Metadata.TotalCost.Equal(decimal.NewFromFloat(0.01)))
	assert.Len(t, loaded.Messages, 1)
}

func TestMemoryStore_LoadNotFound(t *testing.T) {
	store := session.NewMemoryStore()
	_, err := store.Load(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestMemoryStore_SaveNil(t *testing.T) {
	store := session.NewMemoryStore()
	err := store.Save(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session is nil")
}

func TestMemoryStore_Delete(t *testing.T) {
	store := session.NewMemoryStore()
	ctx := context.Background()

	s := makeSession("sess-del")
	require.NoError(t, store.Save(ctx, s))

	require.NoError(t, store.Delete(ctx, "sess-del"))

	_, err := store.Load(ctx, "sess-del")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestMemoryStore_DeleteNotFound(t *testing.T) {
	store := session.NewMemoryStore()
	err := store.Delete(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestMemoryStore_List(t *testing.T) {
	store := session.NewMemoryStore()
	ctx := context.Background()

	// Empty store
	list, err := store.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)

	// Add sessions
	require.NoError(t, store.Save(ctx, makeSession("s1")))
	require.NoError(t, store.Save(ctx, makeSession("s2")))
	require.NoError(t, store.Save(ctx, makeSession("s3")))

	list, err = store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 3)

	ids := make(map[string]bool)
	for _, s := range list {
		ids[s.ID] = true
	}
	assert.True(t, ids["s1"])
	assert.True(t, ids["s2"])
	assert.True(t, ids["s3"])
}

func TestMemoryStore_Fork(t *testing.T) {
	store := session.NewMemoryStore()
	ctx := context.Background()

	original := makeSession("fork-original")
	require.NoError(t, store.Save(ctx, original))

	forked, err := store.Fork(ctx, "fork-original")
	require.NoError(t, err)
	assert.NotEqual(t, "fork-original", forked.ID)
	assert.Len(t, forked.Messages, 1)

	// Both sessions exist in store
	list, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestMemoryStore_ForkNotFound(t *testing.T) {
	store := session.NewMemoryStore()
	_, err := store.Fork(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestMemoryStore_DeepCopyIsolation(t *testing.T) {
	store := session.NewMemoryStore()
	ctx := context.Background()

	s := makeSession("isolate")
	require.NoError(t, store.Save(ctx, s))

	// Mutate original after save
	s.Messages = append(s.Messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock("mutated")))

	loaded, err := store.Load(ctx, "isolate")
	require.NoError(t, err)
	assert.Len(t, loaded.Messages, 1, "store should not be affected by external mutation")

	// Mutate loaded copy
	loaded.Messages = append(loaded.Messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock("mutated-2")))

	loaded2, err := store.Load(ctx, "isolate")
	require.NoError(t, err)
	assert.Len(t, loaded2.Messages, 1, "subsequent load should not see mutations")
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := session.NewMemoryStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		id := "concurrent"
		go func() {
			defer wg.Done()
			_ = store.Save(ctx, makeSession(id))
		}()
		go func() {
			defer wg.Done()
			_, _ = store.Load(ctx, id)
		}()
		go func() {
			defer wg.Done()
			_, _ = store.List(ctx)
		}()
	}
	wg.Wait()
}

func TestMemoryStore_SaveOverwrite(t *testing.T) {
	store := session.NewMemoryStore()
	ctx := context.Background()

	s1 := makeSession("overwrite")
	s1.Metadata.NumTurns = 1
	require.NoError(t, store.Save(ctx, s1))

	s2 := makeSession("overwrite")
	s2.Metadata.NumTurns = 5
	require.NoError(t, store.Save(ctx, s2))

	loaded, err := store.Load(ctx, "overwrite")
	require.NoError(t, err)
	assert.Equal(t, 5, loaded.Metadata.NumTurns, "save should overwrite existing session")
}
