// Package session provides SessionStore implementations for persisting
// agent conversation history.
//
// Available stores:
//   - [MemoryStore] keeps sessions in memory (useful for testing).
//   - [FileStore] persists sessions as JSON files on disk.
//
// Both implement [agent.FullSessionStore] which extends [agent.SessionStore]
// with List and Fork operations.
package session
