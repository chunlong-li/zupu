package handler

import "testing"

func TestHashPasswordBcrypt(t *testing.T) {
	h := hashPassword("secret123")
	if h == "" {
		t.Fatal("hashPassword 返回空")
	}
	if !isBcryptHash(h) {
		t.Fatalf("期望 bcrypt 哈希（60位 $2 开头），实际 %q", h)
	}
	ok, rehash := verifyPassword(h, "secret123")
	if !ok {
		t.Fatal("bcrypt 正确密码应通过")
	}
	if rehash {
		t.Fatal("bcrypt 哈希不应触发迁移")
	}
	ok, _ = verifyPassword(h, "wrong")
	if ok {
		t.Fatal("bcrypt 错误密码不应通过")
	}
}

func TestLegacySHA256Migration(t *testing.T) {
	legacy := legacySHA256("oldpass")
	if !isLegacySHA256(legacy) {
		t.Fatalf("legacySHA256 应为 64 位 hex，实际 %q", legacy)
	}
	ok, rehash := verifyPassword(legacy, "oldpass")
	if !ok {
		t.Fatal("旧 sha256 正确密码应通过")
	}
	if !rehash {
		t.Fatal("旧 sha256 命中应返回 needsRehash=true")
	}
	ok, _ = verifyPassword(legacy, "bad")
	if ok {
		t.Fatal("旧 sha256 错误密码不应通过")
	}
}

func TestIsLegacySHA256(t *testing.T) {
	if isLegacySHA256("$2a$10$abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz") {
		t.Fatal("bcrypt 哈希不应被识别为 legacy sha256")
	}
	if !isLegacySHA256("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855") {
		t.Fatal("64 位 hex 应识别为 legacy sha256")
	}
	if isLegacySHA256("short") {
		t.Fatal("非 64 位不应识别为 legacy")
	}
}
