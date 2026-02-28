package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultDenyPatterns contains shell command patterns that are blocked by default.
// Ported from PicoClaw's 70+ deny patterns.
var DefaultDenyPatterns = []string{
	// Destructive file operations
	`rm\s+(-[a-zA-Z]*)?r`, `rm\s+(-[a-zA-Z]*)?f`,
	`del\s+/[fFsS]`, `rmdir\s+/[sS]`,
	`format\s`, `mkfs\.`,
	`dd\s+if=`, `>\s*/dev/sd`, `>\s*/dev/nvme`,
	// System control
	`shutdown`, `reboot`, `poweroff`, `halt\b`,
	`init\s+[0-6]`, `systemctl\s+(halt|poweroff|reboot)`,
	// Privilege escalation
	`sudo\s`, `su\s+-`, `doas\s`,
	`chmod\s+[0-7]*777`, `chmod\s+(-[a-zA-Z]*)?\+s`,
	`chown\s`, `chgrp\s`,
	// Process killing
	`pkill\s`, `killall\s`, `kill\s+-9`,
	// Command injection
	`\$\(`, `\$\{`, "`",
	`\|\s*sh\b`, `\|\s*bash\b`, `\|\s*zsh\b`,
	`\|\s*dash\b`, `\|\s*ksh\b`,
	`<<\s*EOF`, `<<-\s*EOF`,
	// Dangerous pipes
	`curl\s.*\|\s*(sh|bash)`, `wget\s.*\|\s*(sh|bash)`,
	// Package managers (global install)
	`npm\s+install\s+-g`, `pip\s+install\s+--user`,
	`apt\s+install`, `apt-get\s+install`,
	`yum\s+install`, `dnf\s+install`,
	// Remote access
	`ssh\s`, `scp\s`, `rsync\s.*:`,
	// Git dangerous ops
	`git\s+push\s+(-[a-zA-Z]*)?f`, `git\s+push\s+--force`,
	`git\s+reset\s+--hard`,
	// Docker escape
	`docker\s+run`, `docker\s+exec`,
	// Code execution
	`\beval\s`, `source\s+.*\.sh`,
	// Fork bomb patterns
	`:\(\)\s*\{`, `fork\s*bomb`,
	// Disk operations
	`fdisk\s`, `parted\s`, `lvm\s`,
	// Network
	`iptables\s`, `nft\s`, `ufw\s`,
}

// compiledDenyPatterns caches compiled regexps.
var compiledDenyPatterns []*regexp.Regexp

func init() {
	compiledDenyPatterns = compileDenyPatterns(DefaultDenyPatterns)
}

func compileDenyPatterns(patterns []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		if re, err := regexp.Compile(p); err == nil {
			out = append(out, re)
		}
	}
	return out
}

// ValidateCommand checks a shell command against deny patterns.
// Returns an error if the command matches any blocked pattern.
func ValidateCommand(cmd string, extraDeny []string) error {
	lower := strings.ToLower(cmd)
	for _, re := range compiledDenyPatterns {
		if re.MatchString(lower) {
			return fmt.Errorf("command blocked by security policy: matches pattern %q", re.String())
		}
	}
	if len(extraDeny) > 0 {
		for _, re := range compileDenyPatterns(extraDeny) {
			if re.MatchString(lower) {
				return fmt.Errorf("command blocked by custom policy: matches pattern %q", re.String())
			}
		}
	}
	return nil
}

// ValidatePath ensures a file path stays within the workspace sandbox.
// Prevents path traversal attacks and symlink escapes.
// If allowedPaths is non-empty, absolute paths under those prefixes are also permitted.
func ValidatePath(path, workspace string, allowedPaths ...string) (string, error) {
	if workspace == "" {
		return "", fmt.Errorf("workspace directory not configured")
	}

	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("invalid workspace path: %w", err)
	}

	var fullPath string
	if filepath.IsAbs(path) {
		fullPath = filepath.Clean(path)
	} else {
		fullPath = filepath.Clean(filepath.Join(absWorkspace, path))
	}

	// Check the resolved path stays within workspace or allowed paths.
	inWorkspace := strings.HasPrefix(fullPath, absWorkspace+string(os.PathSeparator)) || fullPath == absWorkspace
	if !inWorkspace {
		if !isUnderAllowedPaths(fullPath, allowedPaths) {
			return "", fmt.Errorf("path %q escapes workspace %q", path, absWorkspace)
		}
	}

	// If the path exists, resolve symlinks and re-check
	if resolved, err := filepath.EvalSymlinks(fullPath); err == nil {
		resolvedInWorkspace := strings.HasPrefix(resolved, absWorkspace+string(os.PathSeparator)) || resolved == absWorkspace
		if !resolvedInWorkspace && !isUnderAllowedPaths(resolved, allowedPaths) {
			return "", fmt.Errorf("symlink %q resolves outside workspace", path)
		}
		return resolved, nil
	}

	// Path doesn't exist yet — validate parent directory
	parent := filepath.Dir(fullPath)
	if resolved, err := filepath.EvalSymlinks(parent); err == nil {
		resolvedInWorkspace := strings.HasPrefix(resolved, absWorkspace+string(os.PathSeparator)) || resolved == absWorkspace
		if !resolvedInWorkspace && !isUnderAllowedPaths(resolved, allowedPaths) {
			return "", fmt.Errorf("parent directory resolves outside workspace")
		}
	}

	return fullPath, nil
}

// isUnderAllowedPaths checks if fullPath is under any of the allowed path prefixes.
func isUnderAllowedPaths(fullPath string, allowedPaths []string) bool {
	for _, ap := range allowedPaths {
		abs, err := filepath.Abs(ap)
		if err != nil {
			continue
		}
		if strings.HasPrefix(fullPath, abs+string(os.PathSeparator)) || fullPath == abs {
			return true
		}
	}
	return false
}

// TenantWorkspaceDir returns the per-tenant workspace directory.
// Empty tenant_id is treated as "default" to prevent falling back to the
// shared root directory, which would leak data across tenants.
func TenantWorkspaceDir(baseDir, tenantID string) string {
	if tenantID == "" {
		tenantID = "default"
	}
	return filepath.Join(baseDir, tenantID)
}

// UserWorkspaceDir returns the per-user workspace directory within a tenant.
// Layout: baseDir/{tenantID}/{userID}
// Falls back to TenantWorkspaceDir when userID is empty.
func UserWorkspaceDir(baseDir, tenantID, userID string) string {
	base := TenantWorkspaceDir(baseDir, tenantID)
	if userID == "" || userID == "default" {
		return base
	}
	return filepath.Join(base, userID)
}

// ValidateWorkspaceCommand checks that paths referenced in a command
// don't escape the workspace directory.
func ValidateWorkspaceCommand(cmd, workspace string) error {
	if workspace == "" {
		return nil
	}
	if strings.Contains(cmd, "../") || strings.Contains(cmd, "..\\") {
		return fmt.Errorf("path traversal detected in command")
	}
	return nil
}
