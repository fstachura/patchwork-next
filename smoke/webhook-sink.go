// Patchwork - automated patch tracking system
// Copyright (C) The Patchwork Contributors (see CONTRIBUTORS)
//
// SPDX-License-Identifier: GPL-2.0-or-later

// Command webhook-sink is a throwaway HTTP server used by the smoke tests to
// receive patchwork webhook deliveries. For every request it recomputes the
// HMAC-SHA256 signature, appends a single JSON line describing the delivery to
// a log file and answers 200. It is meant to be launched with "go run".
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Lshortfile)
	if err := run(); err != nil {
		log.Fatalf("ERROR: %v\n", err)
	}
}

func run() error {
	addr := flag.String("addr", "127.0.0.1:9099", "listen address")
	logPath := flag.String("log", "webhook.log", "path to the delivery log")
	secret := flag.String("secret", "", "HMAC secret used to verify signatures")
	flag.Parse()

	f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("os.OpenFile: %w", err)
	}
	defer f.Close()

	var mu sync.Mutex
	enc := json.NewEncoder(f)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL.Path)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.Body.Close()

		sigOK := false
		if *secret != "" {
			mac := hmac.New(sha256.New, []byte(*secret))
			mac.Write(body)
			want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
			got := r.Header.Get("X-Patchwork-Signature")
			sigOK = hmac.Equal([]byte(want), []byte(got))
		}

		mu.Lock()
		err = enc.Encode(map[string]any{
			"event":    r.Header.Get("X-Patchwork-Event"),
			"delivery": r.Header.Get("X-Patchwork-Delivery"),
			"sig_ok":   sigOK,
			"len":      len(body),
		})
		mu.Unlock()
		if err != nil {
			log.Printf("ERROR: write log: %v\n", err)
		}

		w.WriteHeader(http.StatusOK)
	})

	log.Printf("listening on %s\n", *addr)
	return http.ListenAndServe(*addr, nil)
}
