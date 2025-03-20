package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

const metricsTemplate = `<html>
	<body>
		<h1>Welcome, Chirpy Admin</h1>
		<p>Chirpy has been visited %d times!</p>
	</body>
</html>`

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) metrics() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		w.Write(fmt.Appendf(nil, metricsTemplate, cfg.fileserverHits.Load()))
	})
}

func (cfg *apiConfig) reset() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Store(0)
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
	})
}

func main() {
	apiCfg := apiConfig{}
	serverMux := http.NewServeMux()

	serverMux.Handle(
		"/app/", 
		http.StripPrefix("/app", 
			apiCfg.middlewareMetricsInc(
				http.FileServer(http.Dir(".")),
			),
		),
	)

	serverMux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
	serverMux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)
		type parameters struct {
			Body string `json:"body"`
		}
		params := parameters{}
		if err := decoder.Decode(&params); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid JSON:" + err.Error())
			return
		}
		if len(params.Body) > 140 {
			respondWithError(w, http.StatusBadRequest, "Chirp is too long")
			return
		}

		cleanedBody := replaceBadWords(params.Body)

		type CleanedChirp struct {
			CleanedBody string `json:"cleaned_body"`
		}
		res := CleanedChirp{
			CleanedBody: cleanedBody,
		}
		respondWithJSON(w, http.StatusOK, res)
	})

	serverMux.Handle("GET /admin/metrics", apiCfg.metrics())
	serverMux.Handle("POST /admin/reset", apiCfg.reset())
	

	server := http.Server{
		Addr:    ":8080",
		Handler: serverMux,
	}
	server.ListenAndServe()
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	res, err := json.Marshal(payload)
	if err != nil {
		log.Printf("%s", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(res)
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type JSONError struct {
		Error string `json:"error"`
	}
	jsonErr := JSONError{
		Error: msg,
	}
	respondWithJSON(w, code, jsonErr)
}

func replaceBadWords(s string) string {
	badWords := map[string]bool{"kerfuffle": true, "sharbert": true, "fornax": true}

	words := strings.Split(s, " ")
	for i, word := range words {
		lowered := strings.ToLower(word)
		if badWords[lowered] {
			words[i] = "****"
		}
	}

	return strings.Join(words, " ")
}