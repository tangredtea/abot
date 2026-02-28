// Package session provides persistent session storage implementations for
// ADK-Go's session.Service interface. The JSONL implementation uses
// write-through to disk with in-memory caching and automatic compaction.
package session
