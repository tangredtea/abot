package channels_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"abot/pkg/channels/wecom"
	"abot/pkg/types"
)

// --- test helpers ---

type wecomMockBus struct {
	mu       sync.Mutex
	inbound  []types.InboundMessage
	outbound chan types.OutboundMessage
}

func newWecomMockBus() *wecomMockBus {
	return &wecomMockBus{outbound: make(chan types.OutboundMessage, 16)}
}

func (b *wecomMockBus) PublishInbound(_ context.Context, msg types.InboundMessage) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.inbound = append(b.inbound, msg)
	return nil
}

func (b *wecomMockBus) ConsumeInbound(_ context.Context) (types.InboundMessage, error) {
	return types.InboundMessage{}, nil
}

func (b *wecomMockBus) PublishOutbound(_ context.Context, msg types.OutboundMessage) error {
	b.outbound <- msg
	return nil
}

func (b *wecomMockBus) ConsumeOutbound(ctx context.Context) (types.OutboundMessage, error) {
	select {
	case <-ctx.Done():
		return types.OutboundMessage{}, ctx.Err()
	case msg := <-b.outbound:
		return msg, nil
	}
}

func (b *wecomMockBus) InboundSize() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.inbound)
}

func (b *wecomMockBus) OutboundSize() int { return len(b.outbound) }
func (b *wecomMockBus) Close() error      { return nil }

func (b *wecomMockBus) inboundMessages() []types.InboundMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	cp := make([]types.InboundMessage, len(b.inbound))
	copy(cp, b.inbound)
	return cp
}

// generateTestAESKey returns a 43-char base64 string (32 bytes decoded with "=" appended).
func generateTestAESKey() string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return base64.StdEncoding.EncodeToString(key)[:43]
}

// encryptTestMessage encrypts plaintext using the WeCom message format for testing.
func encryptTestMessage(message, aesKeyB64 string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(aesKeyB64 + "=")
	if err != nil {
		return "", err
	}

	random := make([]byte, 16)
	for i := range random {
		random[i] = byte(i)
	}

	msgBytes := []byte(message)
	receiveID := []byte("test_aibot_id")

	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(msgBytes)))

	plain := append(random, lenBuf...)
	plain = append(plain, msgBytes...)
	plain = append(plain, receiveID...)

	// PKCS7 pad to AES block size
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	plain = append(plain, bytes.Repeat([]byte{byte(pad)}, pad)...)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	ct := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(ct, plain)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// testSignature computes the expected WeCom signature.
func testSignature(token, timestamp, nonce, msgEncrypt string) string {
	params := []string{token, timestamp, nonce, msgEncrypt}
	sort.Strings(params)
	h := sha1.Sum([]byte(strings.Join(params, "")))
	return fmt.Sprintf("%x", h)
}

func validWecomConfig() wecom.WeComConfig {
	return wecom.WeComConfig{
		Token:      "test_token",
		WebhookURL: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=test",
	}
}

// --- crypto tests ---

func TestVerifySignature(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		sig := testSignature("tok", "123", "nonce", "enc")
		if !wecom.VerifySignature("tok", sig, "123", "nonce", "enc") {
			t.Fatal("expected valid signature to pass")
		}
	})
	t.Run("invalid", func(t *testing.T) {
		if wecom.VerifySignature("tok", "bad", "123", "nonce", "enc") {
			t.Fatal("expected invalid signature to fail")
		}
	})
	t.Run("empty token skips", func(t *testing.T) {
		if !wecom.VerifySignature("", "anything", "1", "2", "3") {
			t.Fatal("empty token should skip verification")
		}
	})
}

func TestDecryptMessage(t *testing.T) {
	t.Run("no AES key base64 only", func(t *testing.T) {
		plain := "hello world"
		enc := base64.StdEncoding.EncodeToString([]byte(plain))
		got, err := wecom.DecryptMessage(enc, "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != plain {
			t.Fatalf("got %q, want %q", got, plain)
		}
	})

	t.Run("round trip with AES key", func(t *testing.T) {
		aesKey := generateTestAESKey()
		original := `{"text":"hello"}`
		encrypted, err := encryptTestMessage(original, aesKey)
		if err != nil {
			t.Fatalf("encrypt: %v", err)
		}
		got, err := wecom.DecryptMessage(encrypted, aesKey, "")
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if got != original {
			t.Fatalf("got %q, want %q", got, original)
		}
	})

	t.Run("invalid base64", func(t *testing.T) {
		_, err := wecom.DecryptMessage("!!!invalid!!!", "", "")
		if err == nil {
			t.Fatal("expected error for invalid base64")
		}
	})

	t.Run("invalid AES key", func(t *testing.T) {
		enc := base64.StdEncoding.EncodeToString([]byte("test"))
		_, err := wecom.DecryptMessage(enc, "invalid_key", "")
		if err == nil {
			t.Fatal("expected error for invalid AES key")
		}
	})
}

func TestPKCS7Unpad(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		out, err := wecom.Pkcs7Unpad([]byte{}, 32)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != 0 {
			t.Fatalf("expected empty, got %v", out)
		}
	})

	t.Run("valid padding 3", func(t *testing.T) {
		data := append([]byte("hello"), bytes.Repeat([]byte{3}, 3)...)
		out, err := wecom.Pkcs7Unpad(data, 32)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(out, []byte("hello")) {
			t.Fatalf("got %q, want %q", out, "hello")
		}
	})

	t.Run("zero padding invalid", func(t *testing.T) {
		_, err := wecom.Pkcs7Unpad(append([]byte("test"), 0), 32)
		if err == nil {
			t.Fatal("expected error for zero padding")
		}
	})

	t.Run("padding larger than data", func(t *testing.T) {
		_, err := wecom.Pkcs7Unpad([]byte{20}, 32)
		if err == nil {
			t.Fatal("expected error for padding > data length")
		}
	})
}

// --- dedup cache tests ---

func TestDedupCache(t *testing.T) {
	t.Run("first call returns false, second returns true", func(t *testing.T) {
		dc := wecom.NewDedupCache(time.Minute)
		if dc.Check("a") {
			t.Fatal("first Check should return false")
		}
		if !dc.Check("a") {
			t.Fatal("second Check should return true")
		}
	})

	t.Run("expired entry returns false", func(t *testing.T) {
		dc := wecom.NewDedupCache(10 * time.Millisecond)
		dc.Check("b")
		time.Sleep(20 * time.Millisecond)
		if dc.Check("b") {
			t.Fatal("expired entry should return false")
		}
	})
}

// --- channel creation tests ---

func TestNewWeComChannel(t *testing.T) {
	bus := newWecomMockBus()

	t.Run("missing token", func(t *testing.T) {
		cfg := wecom.WeComConfig{WebhookURL: "https://example.com"}
		_, err := wecom.NewWeComChannel(cfg, bus)
		if err == nil {
			t.Fatal("expected error for missing token")
		}
	})

	t.Run("missing webhook_url", func(t *testing.T) {
		cfg := wecom.WeComConfig{Token: "tok"}
		_, err := wecom.NewWeComChannel(cfg, bus)
		if err == nil {
			t.Fatal("expected error for missing webhook_url")
		}
	})

	t.Run("valid config", func(t *testing.T) {
		ch, err := wecom.NewWeComChannel(validWecomConfig(), bus)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ch.Name() != "wecom" {
			t.Fatalf("Name() = %q, want %q", ch.Name(), "wecom")
		}
		if ch.IsRunning() {
			t.Fatal("new channel should not be running")
		}
	})
}

// --- HandleVerification tests ---

func TestHandleVerification(t *testing.T) {
	bus := newWecomMockBus()
	aesKey := generateTestAESKey()
	cfg := wecom.WeComConfig{
		Token:          "test_token",
		EncodingAESKey: aesKey,
		WebhookURL:     "https://example.com/webhook",
	}
	ch, _ := wecom.NewWeComChannel(cfg, bus)

	t.Run("valid verification", func(t *testing.T) {
		echostr := "test_echostr_123"
		encrypted, _ := encryptTestMessage(echostr, aesKey)
		sig := testSignature("test_token", "123", "nonce", encrypted)

		req := httptest.NewRequest(http.MethodGet,
			"/webhook/wecom?msg_signature="+sig+"&timestamp=123&nonce=nonce&echostr="+encrypted, nil)
		w := httptest.NewRecorder()

		ch.HandleVerification(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if w.Body.String() != echostr {
			t.Fatalf("body = %q, want %q", w.Body.String(), echostr)
		}
	})

	t.Run("missing parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet,
			"/webhook/wecom?msg_signature=sig&timestamp=ts", nil)
		w := httptest.NewRecorder()

		ch.HandleVerification(w, req)

		// Missing nonce+echostr -> 200 (health check probe)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		encrypted, _ := encryptTestMessage("echo", aesKey)
		req := httptest.NewRequest(http.MethodGet,
			"/webhook/wecom?msg_signature=bad&timestamp=1&nonce=n&echostr="+encrypted, nil)
		w := httptest.NewRecorder()

		ch.HandleVerification(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
		}
	})
}

// --- HandleMessageCallback tests ---

func TestHandleMessageCallback(t *testing.T) {
	bus := newWecomMockBus()
	aesKey := generateTestAESKey()
	cfg := wecom.WeComConfig{
		Token:          "test_token",
		EncodingAESKey: aesKey,
		WebhookURL:     "https://example.com/webhook",
	}
	ch, _ := wecom.NewWeComChannel(cfg, bus)

	t.Run("valid message", func(t *testing.T) {
		jsonMsg := `{
			"msgid": "msg_001",
			"aibotid": "test_aibot_id",
			"chattype": "single",
			"from": {"userid": "user123"},
			"response_url": "https://example.com/reply",
			"msgtype": "text",
			"text": {"content": "Hello"}
		}`
		encrypted, _ := encryptTestMessage(jsonMsg, aesKey)

		envelope := struct {
			XMLName xml.Name `xml:"xml"`
			Encrypt string   `xml:"Encrypt"`
		}{Encrypt: encrypted}
		xmlData, _ := xml.Marshal(envelope)

		sig := testSignature("test_token", "1", "n", encrypted)
		req := httptest.NewRequest(http.MethodPost,
			"/webhook/wecom?msg_signature="+sig+"&timestamp=1&nonce=n",
			bytes.NewReader(xmlData))
		w := httptest.NewRecorder()

		ch.HandleMessageCallback(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})
}
