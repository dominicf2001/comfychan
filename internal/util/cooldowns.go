package util

import (
	"sync"
	"time"
)

var (
	PostCooldowns   = make(map[string]time.Time)
	ThreadCooldowns = make(map[string]time.Time)
)

var CooldownMutex sync.RWMutex

const POST_COOLDOWN = 15 * time.Second

const THREAD_COOLDOWN = 2 * time.Minute

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
