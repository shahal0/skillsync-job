package middleware

import (
	"jobservice/client"
	"sync"
)

// authClientCache caches Auth Service clients by address to avoid creating new connections for each request
var (
	authClientCache     = make(map[string]*client.AuthClient)
	authClientCacheMutex sync.RWMutex
)

// getAuthClient gets or creates an Auth Service client for the given address
func getAuthClient(authServiceAddr string) (*client.AuthClient, error) {
	// Check if client already exists in cache
	authClientCacheMutex.RLock()
	c, exists := authClientCache[authServiceAddr]
	authClientCacheMutex.RUnlock()

	if exists {
		return c, nil
	}

	// Create new client if not in cache
	authClientCacheMutex.Lock()
	defer authClientCacheMutex.Unlock()

	// Check again in case another goroutine created it while we were waiting
	c, exists = authClientCache[authServiceAddr]
	if exists {
		return c, nil
	}

	// Create new client
	c, err := client.NewAuthClient(authServiceAddr)
	if err != nil {
		return nil, err
	}

	// Add to cache
	authClientCache[authServiceAddr] = c
	return c, nil
}

