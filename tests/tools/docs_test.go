package tools_test

import (
	"testing"

	"abot/pkg/tools"
)

func TestUpdateDoc_AllowedDocTypes(t *testing.T) {
	allowed := []string{"IDENTITY", "SOUL", "AGENT", "USER"}
	for _, dt := range allowed {
		if !tools.IsAllowedDocType(dt) {
			t.Errorf("expected %q to be allowed", dt)
		}
	}
}

func TestUpdateDoc_RejectedDocTypes(t *testing.T) {
	rejected := []string{"RULES", "TOOLS", "HEARTBEAT", "RANDOM", ""}
	for _, dt := range rejected {
		if tools.IsAllowedDocType(dt) {
			t.Errorf("expected %q to be rejected", dt)
		}
	}
}
