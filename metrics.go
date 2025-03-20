package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

const metricsTemplate = `<html>
	<body>
		<h1>Welcome, Chirpy Admin</h1>
		<p>Chirpy has been visited %d times!</p>
	</body>
</html>`

type apiMetrics struct {
	fileserverHits atomic.Int32
}

func (cfg *apiMetrics) middlewareCountServerHit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiMetrics) metrics() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		w.Write(fmt.Appendf(nil, metricsTemplate, cfg.fileserverHits.Load()))
	})
}

func (cfg *apiMetrics) reset() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Store(0)
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
	})
}