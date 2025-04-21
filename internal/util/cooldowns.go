package util

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	PostCooldowns   = make(map[string]time.Time)
	ThreadCooldowns = make(map[string]time.Time)
)

var cooldownMutex sync.Mutex

const POST_COOLDOWN = 15 * time.Second
const THREAD_COOLDOWN = 2 * time.Minute

func IsOnCooldown(ip string, m map[string]time.Time, duration time.Duration) bool {
	cooldownMutex.Lock()
	defer cooldownMutex.Unlock()

	last, exists := m[ip]
	if !exists || time.Since(last) >= duration {
		m[ip] = time.Now()
		return false
	}
	return true
}

func GetIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Fallback to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func HashIp(ip string) string {
	checksum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(checksum[:])
}
