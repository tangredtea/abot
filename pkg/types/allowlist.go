package types

// AllowlistMode defines how to handle non-allowed senders.
type AllowlistMode string

const (
	AllowlistModeTrigger AllowlistMode = "trigger" // Store message but don't trigger
	AllowlistModeDrop    AllowlistMode = "drop"    // Drop message entirely
)

// AllowlistEntry defines access control for a chat.
type AllowlistEntry struct {
	Allow []string      // User IDs allowed, or ["*"] for all
	Mode  AllowlistMode // How to handle non-allowed senders
}

// SenderAllowlist manages sender access control.
type SenderAllowlist struct {
	Default AllowlistEntry
	Chats   map[string]AllowlistEntry // chatID -> entry
}

// IsAllowed checks if a sender is allowed for a chat.
func (a *SenderAllowlist) IsAllowed(chatID, senderID string) bool {
	entry := a.GetEntry(chatID)
	if len(entry.Allow) == 1 && entry.Allow[0] == "*" {
		return true
	}
	for _, id := range entry.Allow {
		if id == senderID {
			return true
		}
	}
	return false
}

// ShouldDrop returns true if message should be dropped.
func (a *SenderAllowlist) ShouldDrop(chatID, senderID string) bool {
	if a.IsAllowed(chatID, senderID) {
		return false
	}
	entry := a.GetEntry(chatID)
	return entry.Mode == AllowlistModeDrop
}

// GetEntry returns the allowlist entry for a chat.
func (a *SenderAllowlist) GetEntry(chatID string) AllowlistEntry {
	if entry, ok := a.Chats[chatID]; ok {
		return entry
	}
	return a.Default
}
