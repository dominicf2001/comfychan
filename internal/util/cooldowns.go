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

func GetRemainingCooldown(ipHash string, m map[string]time.Time, duration time.Duration) time.Duration {
	CooldownMutex.RLock()
	last, exists := m[ipHash]
	CooldownMutex.RUnlock()
	if !exists {
		return 0
	}
	return duration - time.Since(last)
}

func BeginCooldown(ipHash string, m map[string]time.Time, duration time.Duration) {
	CooldownMutex.Lock()
	defer CooldownMutex.Unlock()

	last, exists := m[ipHash]
	if !exists || time.Since(last) >= duration {
		m[ipHash] = time.Now()
	}
}
