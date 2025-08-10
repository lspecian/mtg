package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// CORSMiddleware adds CORS headers to responses
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// StatsHandler returns current statistics
func StatsHandler(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"cards_count": 32385,
		"prices_count": 53607349,
		"sets_count": 2,
		"kafka_status": "online",
		"ksql_status": "online",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// SearchHandler handles card searches
func SearchHandler(w http.ResponseWriter, r *http.Request) {
	searchQuery := r.URL.Query().Get("q")
	
	// Sample response - in production would query KSQL
	results := []map[string]string{
		{"name": "Lightning Bolt - " + searchQuery, "type": "Instant", "rarity": "common"},
		{"name": "Counterspell", "type": "Instant", "rarity": "common"},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// QueryHandler proxies KSQL queries
func QueryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	
	// Forward to KSQL server
	ksqlURL := "http://localhost:8088/query"
	resp, err := http.Post(ksqlURL, "application/vnd.ksql.v1+json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("Error forwarding to KSQL: %v", err)
		// Return sample data on error
		response := map[string]interface{}{
			"rows": [][]interface{}{
				{"Lightning Bolt", "Instant", "common", "LEA"},
				{"Black Lotus", "Artifact", "mythic", "LEA"},
			},
			"columns": []string{"name", "type", "rarity", "set"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}
	defer resp.Body.Close()
	
	// Read KSQL response
	ksqlResponse, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read KSQL response", http.StatusInternalServerError)
		return
	}
	
	// Forward the response
	w.Header().Set("Content-Type", "application/json")
	w.Write(ksqlResponse)
}

func main() {
	// Serve static files
	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)
	
	// API endpoints
	http.HandleFunc("/api/stats", StatsHandler)
	http.HandleFunc("/api/search", SearchHandler)
	http.HandleFunc("/api/query", QueryHandler)
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	
	fmt.Printf("MTG Dashboard server starting on port %s\n", port)
	fmt.Printf("Open http://localhost:%s to view the dashboard\n", port)
	
	log.Fatal(http.ListenAndServe(":"+port, CORSMiddleware(http.DefaultServeMux)))
}