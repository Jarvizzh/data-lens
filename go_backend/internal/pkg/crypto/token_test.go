package crypto

import (
	"testing"
)

func TestTokenGenerateAndParse(t *testing.T) {
	secretKey := "test-secret-key-123"
	token := GenerateToken(1001, "testuser", "ADMIN", 7, secretKey)
	if token == "" {
		t.Fatal("generated token is empty")
	}

	info := ParseToken(token, secretKey)
	if !info.Valid {
		t.Fatalf("token should be valid, got info: %+v", info)
	}
	if info.UserID != 1001 || info.Username != "testuser" || info.Role != "ADMIN" {
		t.Fatalf("unexpected token info: %+v", info)
	}

	// 错误密钥校验
	invalidInfo := ParseToken(token, "wrong-secret-key")
	if invalidInfo.Valid {
		t.Fatal("token with wrong secret key should be invalid")
	}
}
