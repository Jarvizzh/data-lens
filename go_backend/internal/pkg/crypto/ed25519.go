package crypto

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
)

// GenerateFlicknovelSign 生成番茄司南 OpenAPI Ed25519 签名
// 1. sign 以外的 QueryParam 按参数名升序排序拼接成 key=value&key=value
// 2. 尾部追加 "&body=" + 请求体 JSON
// 3. 用 ed25519 私钥加签该字符串
// 4. 签名用 Base64 URLEncoding 编码
func GenerateFlicknovelSign(queryParams map[string]string, bodyJSON string, privateKeyBase64 string) (string, error) {
	if strings.TrimSpace(privateKeyBase64) == "" {
		return "", fmt.Errorf("private key must not be empty")
	}

	keyBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privateKeyBase64))
	if err != nil {
		return "", fmt.Errorf("invalid base64 private key: %w", err)
	}

	var seed []byte
	if len(keyBytes) == 32 {
		seed = keyBytes
	} else if len(keyBytes) == 64 {
		seed = keyBytes[:32]
	} else {
		return "", fmt.Errorf("ed25519 private key length must be 32 or 64 bytes, got: %d", len(keyBytes))
	}

	privKey := ed25519.NewKeyFromSeed(seed)
	canonicalMessage := BuildCanonicalMessage(queryParams, bodyJSON)

	sig := ed25519.Sign(privKey, []byte(canonicalMessage))
	return base64.URLEncoding.EncodeToString(sig), nil
}

// BuildCanonicalMessage 构建待签名规范化消息
func BuildCanonicalMessage(queryParams map[string]string, bodyJSON string) string {
	keys := make([]string, 0, len(queryParams))
	for k := range queryParams {
		if !strings.EqualFold(k, "sign") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString("&")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(queryParams[k])
	}

	if sb.Len() > 0 {
		sb.WriteString("&")
	}
	sb.WriteString("body=")
	sb.WriteString(bodyJSON)

	return sb.String()
}
