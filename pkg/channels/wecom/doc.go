// Package wecom implements the WeCom (Enterprise WeChat) channel adapter.
// It runs an HTTP webhook server, handles AES-CBC message decryption,
// signature verification, and 5-minute TTL message deduplication.
package wecom
