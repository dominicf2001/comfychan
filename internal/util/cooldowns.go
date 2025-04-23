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

var CooldownMutex sync.RWMutex

const POST_COOLDOWN = 15 * time.Second

// const POST_COOLDOWN = 0 * time.Second

const THREAD_COOLDOWN = 2 * time.Minute

// const THREAD_COOLDOWN = 0 * time.Minute

func GetRemainingCooldown(ip string, m map[string]time.Time, duration time.Duration) time.Duration {
	CooldownMutex.RLock()
	last, exists := m[ip]
	CooldownMutex.RUnlock()
	if !exists {
		return 0
	}
	return duration - time.Since(last)
}

func BeginCooldown(ip string, m map[string]time.Time, duration time.Duration) {
	CooldownMutex.Lock()
	defer CooldownMutex.Unlock()

	last, exists := m[ip]
	if !exists || time.Since(last) >= duration {
		m[ip] = time.Now()
	}
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
