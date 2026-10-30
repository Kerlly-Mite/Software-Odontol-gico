package main

import (
	"testing"
	"time"
)

func TestAttemptLimiterBlocksAfterFiveFailures(t *testing.T) {
	l := &attemptLimiter{fails: map[string][]time.Time{}}
	for i := 0; i < 5; i++ {
		l.fail("login:a@sigo.local")
	}
	if !l.blocked("login:a@sigo.local", 5, time.Minute) {
		t.Fatal("tras 5 fallos la cuenta debe bloquearse temporalmente")
	}
	if l.blocked("login:otra@sigo.local", 5, time.Minute) {
		t.Fatal("otra cuenta no debe verse afectada")
	}
	l.reset("login:a@sigo.local")
	if l.blocked("login:a@sigo.local", 5, time.Minute) {
		t.Fatal("un acceso correcto debe reiniciar el contador")
	}
}
