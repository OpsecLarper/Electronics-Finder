package main

import (
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

// In browser mode the app has no window to close, so it exits once the
// page stops sending heartbeats (tab closed).
var lastPing atomic.Int64

func handlePing(w http.ResponseWriter, r *http.Request) {
	lastPing.Store(time.Now().Unix())
	w.WriteHeader(http.StatusNoContent)
}

func waitForever() {
	start := time.Now()
	for {
		time.Sleep(5 * time.Second)
		lp := lastPing.Load()
		if lp == 0 {
			if time.Since(start) > 3*time.Minute {
				os.Exit(0)
			}
			continue
		}
		if time.Since(time.Unix(lp, 0)) > 45*time.Second {
			os.Exit(0)
		}
	}
}
