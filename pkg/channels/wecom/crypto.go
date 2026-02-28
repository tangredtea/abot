package wecom

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

// VerifySignature verifies the WeCom message signature.
// SHA1(sort(token, timestamp, nonce, msgEncrypt))
func VerifySignature(token, msgSignature, timestamp, nonce, msgEncrypt string) bool {
	if token == "" {
		return true // skip verification if token is not configured
	}
	params := []string{token, timestamp, nonce, msgEncrypt}
	sort.Strings(params)
	hash := sha1.Sum([]byte(strings.Join(params, "")))
	expected := fmt.Sprintf("%x", hash)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(msgSignature)) == 1
}

// DecryptMessage decrypts a WeCom encrypted message.
// Format after AES-CBC decrypt + PKCS7 unpad:
//
//	random(16) + msgLen(4 big-endian) + msg + receiveid
func DecryptMessage(encryptedMsg, encodingAESKey, receiveid string) (string, error) {
	if encodingAESKey == "" {
		decoded, err := base64.StdEncoding.DecodeString(encryptedMsg)
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}

	aesKey, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return "", fmt.Errorf("decode AES key: %w", err)
	}

	cipherText, err := base64.StdEncoding.DecodeString(encryptedMsg)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	if len(cipherText) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	iv := aesKey[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	plain := make([]byte, len(cipherText))
	mode.CryptBlocks(plain, cipherText)

	plain, err = Pkcs7Unpad(plain, 32)
	if err != nil {
		return "", fmt.Errorf("unpad: %w", err)
	}

	if len(plain) < 20 {
		return "", fmt.Errorf("decrypted payload too short")
	}
	msgLen := binary.BigEndian.Uint32(plain[16:20])
	if int(msgLen) > len(plain)-20 {
		return "", fmt.Errorf("invalid message length")
	}

	msg := plain[20 : 20+msgLen]

	if receiveid != "" && len(plain) > 20+int(msgLen) {
		actual := string(plain[20+msgLen:])
		if actual != receiveid {
			return "", fmt.Errorf("receiveid mismatch: want %s, got %s", receiveid, actual)
		}
	}

	return string(msg), nil
}

// errInvalidPadding is a generic error to prevent padding oracle attacks.
// All padding validation failures return the same error message.
var errInvalidPadding = fmt.Errorf("decryption failed")

// Pkcs7Unpad removes PKCS#7 padding. WeCom uses blockSize 32.
func Pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return nil, errInvalidPadding
	}
	for i := 0; i < padding; i++ {
		if data[len(data)-1-i] != byte(padding) {
			return nil, errInvalidPadding
		}
	}
	return data[:len(data)-padding], nil
}
