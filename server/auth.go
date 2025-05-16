package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

type UserCredentials struct {
	Username     string
	PasswordHash string
}

type AuthManager struct {
	users map[string]UserCredentials
	mu    sync.RWMutex
}

func NewAuthManager() *AuthManager {
	return &AuthManager{
		users: make(map[string]UserCredentials),
	}
}

func (am *AuthManager) RegisterUser(username, password string) bool {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.users[username]; exists {
		return false
	}

	hash := sha256.Sum256([]byte(password))
	hashString := hex.EncodeToString(hash[:])

	am.users[username] = UserCredentials{
		Username:     username,
		PasswordHash: hashString,
	}

	return true
}

func (am *AuthManager) AuthenticateUser(username, password string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	credentials, exists := am.users[username]
	if !exists {
		return false
	}

	hash := sha256.Sum256([]byte(password))
	hashString := hex.EncodeToString(hash[:])

	return credentials.PasswordHash == hashString
}
