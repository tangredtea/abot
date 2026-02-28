---
name: memory
description: Two-layer memory system with vector-based recall. Use when the agent needs to save important facts, search past events, or manage long-term and user-level memory.
always: true
---

# Memory

## Structure

- **Tenant-level MEMORY** — Long-term facts shared across all users in a tenant (project context, rules, relationships). Always loaded into context via workspace.
- **User-level MEMORY** — Per-user facts (preferences, personal context). Loaded when the user is identified.
- **Vector store** — Searchable event log. NOT loaded into context. Search it with `search_memory`.

## Searching Past Events

Use the `search_memory` capability provided by `tool.Context` to perform semantic search over past conversations and events. Results are filtered by tenant and optionally by user.

## When to Save Memory

Write important facts immediately using workspace update tools:

- User preferences ("I prefer dark mode")
- Project context ("The API uses OAuth2")
- Relationships ("Alice is the project lead")
- Decisions ("We chose PostgreSQL over MySQL for X reason")

Tenant-level facts go to the tenant MEMORY doc. User-specific facts go to the user MEMORY doc.

## Auto-consolidation

Old conversations are automatically summarized when the session grows large. Long-term facts are extracted to the appropriate MEMORY doc. Searchable logs are vectorized for semantic recall. You don't need to manage this.
