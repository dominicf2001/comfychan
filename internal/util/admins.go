package util

import (
	"sync"
	"time"
)

type AdminSession struct {
	Expiration time.Time
	Username   string
}

var (
	AdminSessions = make(map[string]AdminSession)
	AdminMutex    = sync.RWMutex{}
)

func IsAdminSessionValid(token string) bool {
	AdminMutex.RLock()
	session, exists := AdminSessions[token]
	AdminMutex.RUnlock()

	if !exists || time.Now().After(session.Expiration) {
		AdminMutex.Lock()
		delete(AdminSessions, token)
		AdminMutex.Unlock()
		return false
	}

	return true
}

func CreateAdminSession(token string, session AdminSession) {
	AdminMutex.Lock()
	AdminSessions[token] = session
	AdminMutex.Unlock()
}

func DeleteAdminSession(token string) {
	AdminMutex.Lock()
	delete(AdminSessions, token)
	AdminMutex.Unlock()
}

func HasExistingAdminSession(username string) bool {
	AdminMutex.RLock()
	defer AdminMutex.RUnlock()
	for _, session := range AdminSessions {
		if session.Username == username {
			return true
		}
	}

	return false
}
