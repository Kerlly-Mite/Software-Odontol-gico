package main

import (
	"sync"
	"time"
)

// attemptLimiter limita los intentos fallidos de acceso por clave (correo) dentro de una ventana de tiempo.
type attemptLimiter struct {
	mu    sync.Mutex
	fails map[string][]time.Time
}

var loginLimiter = &attemptLimiter{fails: map[string][]time.Time{}}

func (l *attemptLimiter) blocked(key string, maxFails int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-window)
	kept := []time.Time{}
	for _, t := range l.fails[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	l.fails[key] = kept
	return len(kept) >= maxFails
}

func (l *attemptLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[key] = append(l.fails[key], time.Now())
}

func (l *attemptLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
}
