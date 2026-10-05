package crypto

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TokenInfo 存储解析后的用户 Token 信息
type TokenInfo struct {
	UserID   int64  `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Valid    bool   `json:"valid"`
}

// GenerateToken 生成包含有效期的加密 Token (与 Java TokenUtil 100% 兼容)
// 结构: Base64Url(userId:username:role:expireTimestamp:signature)
func GenerateToken(userID int64, username, role string, expireDays int, secretKey string) string {
	expireTime := time.Now().UnixMilli() + int64(expireDays)*24*3600*1000
	rawPayload := fmt.Sprintf("%d:%s:%s:%d", userID, username, role, expireTime)
	signature := sign(rawPayload, secretKey)
	fullToken := rawPayload + ":" + signature
	return base64.URLEncoding.EncodeToString([]byte(fullToken))
}

// ParseToken 解析并验证 Token 是否合法
func ParseToken(tokenStr, secretKey string) TokenInfo {
	tokenStr = strings.TrimSpace(tokenStr)
	if tokenStr == "" {
		return TokenInfo{Valid: false}
	}

	decodedBytes, err := base64.URLEncoding.DecodeString(tokenStr)
	if err != nil {
		// 容错: 尝试带 Padding 或无 Padding
		decodedBytes, err = base64.RawURLEncoding.DecodeString(tokenStr)
		if err != nil {
			return TokenInfo{Valid: false}
		}
	}

	parts := strings.Split(string(decodedBytes), ":")
	nowMs := time.Now().UnixMilli()

	// 新版 5 字段: userId:username:role:expireTime:signature
	if len(parts) == 5 {
		uid, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return TokenInfo{Valid: false}
		}
		username := parts[1]
		role := parts[2]
		exp, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return TokenInfo{Valid: false}
		}
		sig := parts[4]

		if nowMs > exp {
			return TokenInfo{UserID: uid, Username: username, Role: role, Valid: false}
		}

		expectedSig := sign(fmt.Sprintf("%d:%s:%s:%d", uid, username, role, exp), secretKey)
		if expectedSig == sig {
			return TokenInfo{UserID: uid, Username: username, Role: role, Valid: true}
		}
	}

	// 兼容旧版 3 字段: username:expireTime:signature
	if len(parts) == 3 {
		username := parts[0]
		exp, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return TokenInfo{Valid: false}
		}
		sig := parts[2]

		if nowMs > exp {
			return TokenInfo{UserID: 1, Username: username, Role: "ADMIN", Valid: false}
		}

		expectedSig := sign(fmt.Sprintf("%s:%d", username, exp), secretKey)
		if expectedSig == sig {
			return TokenInfo{UserID: 1, Username: username, Role: "ADMIN", Valid: true}
		}
	}

	return TokenInfo{Valid: false}
}

func sign(payload, secretKey string) string {
	hash := sha256.Sum256([]byte(payload + secretKey))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}
