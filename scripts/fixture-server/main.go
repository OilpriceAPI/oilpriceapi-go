package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
)

func main() {
	urlFile := flag.String("url-file", "", "path used to publish the fixture URL")
	flag.Parse()
	if *urlFile == "" {
		fmt.Fprintln(os.Stderr, "-url-file is required")
		os.Exit(2)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(*urlFile, []byte("http://"+listener.Addr().String()), 0o600); err != nil {
		panic(err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/v1/prices/latest" || r.URL.Query().Get("by_code") != "BRENT_CRUDE_USD" {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "fixture route not found"})
			return
		}

		switch r.Header.Get("Authorization") {
		case "Token valid-smoke-key":
			_, _ = w.Write([]byte(`{"status":"success","data":{"code":"BRENT_CRUDE_USD","price":71.80,"currency":"USD","unit":"barrel","source":"market_reporting","created_at":"2026-07-19T12:00:00Z","updated_at":"2026-07-19T12:00:00Z"}}`))
		case "Token invalid-smoke-key":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid API key"}`))
		case "Token locked-smoke-key":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"dataset not enabled"}`))
		case "Token limited-smoke-key":
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"request limit reached"}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"missing API key"}`))
		}
	})

	if err := http.Serve(listener, handler); err != nil {
		panic(err)
	}
}
