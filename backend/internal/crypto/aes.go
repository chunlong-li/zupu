package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
)

// GenerateKey 生成 32 字节 AES-256 随机密钥
func GenerateKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// DecodeKey 将 hex 编码的密钥转为 []byte
func DecodeKey(hexKey string) ([]byte, error) {
	return hex.DecodeString(hexKey)
}

// Encrypt AES-256-GCM 加密，返回 hex 编码的密文（含 nonce）
func Encrypt(plaintext, hexKey string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	key, err := DecodeKey(hexKey)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// Decrypt AES-256-GCM 解密，输入 hex 编码的密文
func Decrypt(cipherHex, hexKey string) (string, error) {
	if cipherHex == "" {
		return "", nil
	}
	key, err := DecodeKey(hexKey)
	if err != nil {
		return "", err
	}
	data, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}