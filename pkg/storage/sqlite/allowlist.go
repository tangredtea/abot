package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"

	"abot/pkg/types"
)

type AllowlistStore struct {
	db *sql.DB
}

func NewAllowlistStore(db *sql.DB) *AllowlistStore {
	return &AllowlistStore{db: db}
}

func (s *AllowlistStore) GetEntry(ctx context.Context, tenantID, chatID string) (*types.AllowlistEntry, error) {
	var entryJSON string
	err := s.db.QueryRowContext(ctx, "SELECT entry FROM sender_allowlist WHERE tenant_id = ? AND chat_id = ?", tenantID, chatID).Scan(&entryJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entry types.AllowlistEntry
	_ = json.Unmarshal([]byte(entryJSON), &entry)
	return &entry, nil
}

func (s *AllowlistStore) SetEntry(ctx context.Context, tenantID, chatID string, entry *types.AllowlistEntry) error {
	data, _ := json.Marshal(entry)
	_, err := s.db.ExecContext(ctx, "INSERT OR REPLACE INTO sender_allowlist (tenant_id, chat_id, entry) VALUES (?, ?, ?)", tenantID, chatID, data)
	return err
}

func (s *AllowlistStore) DeleteEntry(ctx context.Context, tenantID, chatID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sender_allowlist WHERE tenant_id = ? AND chat_id = ?", tenantID, chatID)
	return err
}

func (s *AllowlistStore) ListEntries(ctx context.Context, tenantID string) (map[string]types.AllowlistEntry, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT chat_id, entry FROM sender_allowlist WHERE tenant_id = ?", tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]types.AllowlistEntry)
	for rows.Next() {
		var chatID, entryJSON string
		if err := rows.Scan(&chatID, &entryJSON); err != nil {
			return nil, err
		}
		var entry types.AllowlistEntry
		_ = json.Unmarshal([]byte(entryJSON), &entry)
		result[chatID] = entry
	}
	return result, rows.Err()
}

var _ types.AllowlistStore = (*AllowlistStore)(nil)
