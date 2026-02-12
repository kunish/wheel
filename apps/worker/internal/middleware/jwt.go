package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// JWTPayload matches the TS JWT payload structure exactly.
type JWTPayload struct {
	IAT int64  `json:"iat"`
	EXP int64  `json:"exp"`
	ISS string `json:"iss"`
}

func base64UrlEncode(data []byte) string {
	s := base64.RawURLEncoding.EncodeToString(data)
	return s
}

func base64UrlDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// SignJWT produces an HS256 JWT compatible with the TypeScript implementation.
func SignJWT(payload JWTPayload, secret string) (string, error) {
	header := base64UrlEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encodedBody := base64UrlEncode(body)
	data := header + "." + encodedBody

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	sig := base64UrlEncode(mac.Sum(nil))

	return data + "." + sig, nil
}

// VerifyJWT verifies an HS256 JWT and returns the payload.
func VerifyJWT(token, secret string) (*JWTPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	data := parts[0] + "." + parts[1]
	sig, err := base64UrlDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	expected := mac.Sum(nil)

	if !hmac.Equal(sig, expected) {
		return nil, fmt.Errorf("invalid signature")
	}

	body, err := base64UrlDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding")
	}

	var payload JWTPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("invalid payload JSON")
	}

	if payload.EXP > 0 && payload.EXP < time.Now().Unix() {
		return nil, fmt.Errorf("token expired")
	}

	return &payload, nil
}

// GenerateToken creates a JWT payload and expireAt string.
// expiresMinutes: 0 = 15 minutes, -1 = 30 days, other = custom minutes.
func GenerateToken(expiresMinutes int) (JWTPayload, string) {
	now := time.Now().Unix()
	var exp int64
	switch {
	case expiresMinutes == 0:
		exp = now + 15*60
	case expiresMinutes == -1:
		exp = now + 30*24*60*60
	default:
		exp = now + int64(expiresMinutes)*60
	}

	payload := JWTPayload{IAT: now, EXP: exp, ISS: "wheel"}
	expireAt := time.Unix(exp, 0).UTC().Format(time.RFC3339)
	return payload, expireAt
}

// JWTAuth is a Gin middleware that validates Bearer JWT tokens.
func JWTAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
			c.Abort()
			return
		}

		token := authHeader[7:]
		_, err := VerifyJWT(token, jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid or expired token"})
			c.Abort()
			return
		}

		c.Next()
	}
}
