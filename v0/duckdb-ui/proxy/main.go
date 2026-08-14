// duckdb-ui-proxy sits between Traefik and DuckDB's own `-ui` HTTP server.
//
// The `ui` extension hardcodes its CSRF guard to accept only
// Origin/Referer == "http://localhost:<port>" (see duckdb/duckdb-ui
// src/http_server.cpp) — a deliberate check, not a bug, and not
// configurable. That's incompatible with reaching the console through a
// real domain behind a reverse proxy. Rather than patching and
// self-compiling the extension (a large vendored C++ engine, high
// maintenance burden), this proxy rewrites the Origin/Referer headers on
// the way IN to look like the real server's own localhost, then forwards
// untouched. Auth (Authelia) already sits in front of this in Traefik, so
// the CSRF guard's job — stopping arbitrary third-party pages from driving
// the console via a victim's browser — is still done; we're just relocating
// where "trusted origin" is decided.
package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

func main() {
	listenAddr := getenv("DUCKDB_UI_PROXY_LISTEN", "[::1]:4214")
	backendURL := getenv("DUCKDB_UI_PROXY_BACKEND", "http://[::1]:4213")
	localOrigin := getenv("DUCKDB_UI_PROXY_LOCAL_ORIGIN", "http://localhost:4213")

	backend, err := url.Parse(backendURL)
	if err != nil {
		log.Fatalf("invalid backend URL %q: %v", backendURL, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(backend)
	proxy.FlushInterval = -1 // stream responses (e.g. /localEvents) instead of buffering

	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		if req.Header.Get("Origin") != "" {
			req.Header.Set("Origin", localOrigin)
		}
		if req.Header.Get("Referer") != "" {
			req.Header.Set("Referer", localOrigin+"/")
		}
	}

	log.Printf("duckdb-ui-proxy: listening on %s, forwarding to %s with Origin/Referer rewritten to %s", listenAddr, backend, localOrigin)
	if err := http.ListenAndServe(listenAddr, proxy); err != nil {
		log.Fatal(err)
	}
}

func getenv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
