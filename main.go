package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"slices"
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
	secret string
	polka_key string
	port string
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
		secret: os.Getenv("SECRET"),
		polka_key: os.Getenv("POLKA_KEY"),
		port: os.Getenv("PORT"),
	}

	apiMetrics := apiMetrics{}
	serverMux := http.NewServeMux()

	serverMux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

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

		accessToken, err := auth.MakeJWT(user.ID, apiCfg.secret)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		refreshToken := auth.MakeRefreshToken()

		_, err = apiCfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
			Token: refreshToken,
			UserID: user.ID,
			ExpiresAt: time.Now().UTC().Add(time.Hour * 24 * 60),
		})
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		responseBody := map[string]any{
			"id": user.ID,
			"created_at": user.CreatedAt,
			"updated_at": user.UpdatedAt,
			"email": user.Email,
			"token": accessToken,
			"refresh_token": refreshToken,
			"is_chirpy_red": user.IsChirpyRed,
		}

		respondWithJSON(w, http.StatusOK, responseBody)
	})
	serverMux.HandleFunc("POST /api/refresh", func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}

		user_id, err := apiCfg.db.GetUserFromRefreshToken(r.Context(), token)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}

		jwt, err := auth.MakeJWT(user_id, apiCfg.secret)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondWithJSON(w, http.StatusOK, map[string]any{
			"token": jwt,
		})
	})
	serverMux.HandleFunc("POST /api/revoke", func(w http.ResponseWriter, r *http.Request) {
		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}

		err = apiCfg.db.RevokeRefreshToken(r.Context(), token)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		
		respondWithJSON(w, http.StatusNoContent, nil)
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
	serverMux.HandleFunc("POST /api/polka/webhooks", func(w http.ResponseWriter, r *http.Request) {
		apiKey, err := auth.GetAPIKey(r.Header)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}
		if apiKey != apiCfg.polka_key {
			respondWithError(w, http.StatusUnauthorized, "Invalid API key")
			return
		}
		
		type parameters struct {
			Event string `json:"event"`
			Data struct {
				ID string `json:"user_id"`
			}`json:"data"`
		}

		decoder := json.NewDecoder(r.Body)
		params := parameters{}
		if err := decoder.Decode(&params); err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid JSON:" + err.Error())
			return
		}

		if params.Event != "user.upgraded" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		userUUID, err := uuid.Parse(params.Data.ID)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid UUID:" + err.Error())
			return
		}

		err = apiCfg.db.ActivateChirpyRed(r.Context(), userUUID)
		if err != nil {
			respondWithError(w, http.StatusNotFound, err.Error())
			return
		}

		respondWithJSON(w, http.StatusNoContent, nil)
	})

	serverMux.HandleFunc("GET /api/chirps", func(w http.ResponseWriter, r *http.Request) {
		var chirps []database.Chirp
		author_id := r.URL.Query().Get("author_id")
		if author_id != "" {
			userUUID, err := uuid.Parse(author_id)
			if err != nil {
				respondWithError(w, http.StatusBadRequest, "Invalid UUID:" + err.Error())
				return
			}
			
			chirps, err = apiCfg.db.GetChirpsByAuthor(r.Context(), userUUID)
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
		} else {
			chirps, err = apiCfg.db.GetAllChirps(r.Context())
			if err != nil {
				respondWithError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}

		sortOption := r.URL.Query().Get("sort")
		if sortOption == "desc" {
			slices.SortFunc(chirps, func(a, b database.Chirp) int {
				return b.CreatedAt.Compare(a.CreatedAt)
			})
		} else {
			slices.SortFunc(chirps, func(a, b database.Chirp) int {
				return a.CreatedAt.Compare(b.CreatedAt)
			})
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
		token, err := auth.GetBearerToken(r.Header)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}
		userID, err := auth.ValidateJWT(token, apiCfg.secret)
		if err != nil {
			respondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}
		
		type parameters struct {
			Body string `json:"body"`
		}

		decoder := json.NewDecoder(r.Body)
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
		Addr:    ":" + apiCfg.port,
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