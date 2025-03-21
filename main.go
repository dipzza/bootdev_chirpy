package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dipzza/bootdev_chirpy/internal/auth"
	"github.com/dipzza/bootdev_chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	db *database.Queries
	platform string
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal(err)
	}
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}

	dbQueries := database.New(db)
	apiCfg := apiConfig{
		db: dbQueries,
		platform: os.Getenv("PLATFORM"),
	}

	apiMetrics := apiMetrics{}
	serverMux := http.NewServeMux()

	serverMux.Handle("GET /admin/metrics", apiMetrics.metrics())
	serverMux.Handle("POST /admin/reset", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiCfg.platform != "dev" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		
		apiCfg.db.DeleteAllUsers(r.Context())
		apiMetrics.reset().ServeHTTP(w, r)
	}))
	serverMux.Handle("/app/", http.StripPrefix("/app", 
			apiMetrics.middlewareCountServerHit(http.FileServer(http.Dir("."))),
		),
	)

	serverMux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(200)
		w.Write([]byte("OK"))
	})
	serverMux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		type parameters struct {
			Email string `json:"email"`
			Password string `json:"password"`
		}

		decoder := json.NewDecoder(r.Body)
		params := parameters{}
		if err := decoder.Decode(&params); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid JSON:" + err.Error())
			return
		}
		user, err := apiCfg.db.GetUser(r.Context(), params.Email)
		authorized := err == nil && auth.CheckPasswordHash(params.Password, user.HashedPassword)
		if !authorized {
			respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
			return
		}

		type UserResponse struct {
			ID             uuid.UUID `json:"id"`
			CreatedAt      time.Time `json:"created_at"`
			UpdatedAt      time.Time `json:"updated_at"`
			Email          string    `json:"email"`
		}
		userResponse := UserResponse{
			ID: user.ID,
			Email: user.Email,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		}

		respondWithJSON(w, http.StatusOK, userResponse)
	})
	serverMux.HandleFunc("POST /api/users", func(w http.ResponseWriter, r *http.Request) {
		type parameters struct {
			Email string `json:"email"`
			Password string `json:"password"`
		}

		decoder := json.NewDecoder(r.Body)
		params := parameters{}
		if err := decoder.Decode(&params); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid JSON:" + err.Error())
			return
		}
		hashedPassword, err := auth.HashPassword(params.Password)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		user, err := apiCfg.db.CreateUser(r.Context(), database.CreateUserParams{
			Email: params.Email,
			HashedPassword: hashedPassword,
		})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondWithJSON(w, http.StatusCreated, user)
	})

	serverMux.HandleFunc("GET /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		chirps, err := apiCfg.db.GetAllChirps(r.Context())
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondWithJSON(w, http.StatusOK, chirps)
	})
	serverMux.HandleFunc("GET /api/chirps/{id}", func(w http.ResponseWriter, r *http.Request) {
		userUUID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid UUID:" + err.Error())
			return
		}
		
		chirp, err := apiCfg.db.GetChirp(r.Context(), userUUID)
		if err != nil {
			respondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, chirp)
	})
	serverMux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		type parameters struct {
			Body string `json:"body"`
			UserId string `json:"user_id"`
		}

		decoder := json.NewDecoder(r.Body)
		params := parameters{}
		if err := decoder.Decode(&params); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid JSON:" + err.Error())
			return
		}
		userID, err := uuid.Parse(params.UserId)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid UUID:" + err.Error())
			return
		}
		if len(params.Body) > 140 {
			respondWithError(w, http.StatusBadRequest, "Chirp is too long")
			return
		}
		cleanedBody := replaceBadWords(params.Body)

		chirp, err := apiCfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
			Body: cleanedBody,
			UserID: userID,
		})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondWithJSON(w, http.StatusCreated, chirp)
	})

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