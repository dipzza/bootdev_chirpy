package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func main() {
	apiMetrics := apiMetrics{}
	serverMux := http.NewServeMux()

	serverMux.Handle("/app/", http.StripPrefix("/app", 
			apiMetrics.middlewareCountServerHit(http.FileServer(http.Dir("."))),
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

	serverMux.Handle("GET /admin/metrics", apiMetrics.metrics())
	serverMux.Handle("POST /admin/reset", apiMetrics.reset())

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