package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

var (
	ctx = context.Background()
	rdb *redis.Client
)

type Player struct {
	Username string `json:"username"`
	Score    int    `json:"score"`
}

type LoginRequest struct {
	Username string `json:"username"`
}

type CardDraw struct {
	Card string `json:"cardType"`
}

type GameState struct {
	Deck      []string `json:"deck"`
	HasDefuse bool     `json:"has_defuse"`
}

func init() {
	_ = godotenv.Load()

	redis_address := os.Getenv("ADDRESS")
	redis_pass := os.Getenv("PASSWORD")
	rdb = redis.NewClient(&redis.Options{
		Addr:     redis_address,
		Password: redis_pass,
		DB:       0,
	})
}

func main() {
	r := mux.NewRouter()

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})

	r.HandleFunc("/api/login", handleLogin).Methods("POST")
	r.HandleFunc("/api/score", updateScore).Methods("POST")
	r.HandleFunc("/api/leaderboard", getLeaderboard).Methods("GET")
	r.HandleFunc("/api/saveCardDraw", saveCardDraw).Methods("POST")
	r.HandleFunc("/api/deleteSavedCards", deleteSavedCards).Methods("DELETE")
	r.HandleFunc("/api/fetchSavedCards", fetchSavedCards).Methods("GET")

	handler := c.Handler(r)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := rdb.Get(ctx, "user:"+req.Username).Result()
	if err == redis.Nil {
		err = rdb.Set(ctx, "user:"+req.Username, 0, 0).Err()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func updateScore(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := rdb.Incr(ctx, "user:"+req.Username).Err()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	cardKey := fmt.Sprintf("game:%s:cards", username)

	er := rdb.Del(ctx, cardKey).Err()
	if er != nil {
		http.Error(w, "Error deleting saved cards", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func getLeaderboard(w http.ResponseWriter, r *http.Request) {
	keys, err := rdb.Keys(ctx, "user:*").Result()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var players []Player
	for _, key := range keys {
		username := key[5:]
		score, err := rdb.Get(ctx, key).Int()
		if err != nil {
			continue
		}
		players = append(players, Player{
			Username: username,
			Score:    score,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(players)
}

func saveCardDraw(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	var draw CardDraw
	if err := json.NewDecoder(r.Body).Decode(&draw); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	cardKey := fmt.Sprintf("game:%s:cards", username)
	err := rdb.LPush(ctx, cardKey, draw.Card).Err()
	if err != nil {
		http.Error(w, "Error saving card draw", http.StatusInternalServerError)
		return
	}
	printSavedCards(cardKey)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Card draw saved successfully"})
}

func printSavedCards(cardKey string) {
	cards, err := rdb.LRange(ctx, cardKey, 0, -1).Result()
	if err != nil {
		fmt.Printf("Error retrieving saved cards: %v\n", err)
		return
	}

	fmt.Printf("Current cards for key %s: %v\n", cardKey, cards)
}

func deleteSavedCards(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	cardKey := fmt.Sprintf("game:%s:cards", username)

	err := rdb.Del(ctx, cardKey).Err()
	if err != nil {
		http.Error(w, "Error deleting saved cards", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func fetchSavedCards(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	cardKey := fmt.Sprintf("game:%s:cards", username)

	cards, err := rdb.LRange(ctx, cardKey, 0, -1).Result()
	if err != nil {
		http.Error(w, "Error fetching saved cards", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(cards)
}
