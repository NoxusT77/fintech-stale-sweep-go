package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	client, err := NewInfraClient()
	if err != nil {
		log.Fatal(err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/sweep", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		var events []PaymentEvent
		if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
			http.Error(w, "invalid payment events", 400)
			return
		}
		notices := decide(events, time.Now().UTC(), 24*time.Hour)
		for _, n := range notices {
			if err := client.QueuePublish(n); err != nil {
				http.Error(w, err.Error(), 502)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"queued": len(notices), "audit": notices})
	})
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		id, err := client.CronCreate("0 * * * *", "https://"+r.Host+"/sweep")
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		fmt.Fprintf(w, "job_id=%s\n", id)
	})
	log.Printf("stale sweep listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
