package main

import (
	"cicd-go-k8s/internal/handlers"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", handlers.Health)
	mux.HandleFunc("/ready", handlers.Ready)
	mux.HandleFunc("/api/hello", handlers.Hello)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Println("now listening on port 8080")

	err := http.ListenAndServe(":8080", server.Handler)

	if err != nil {
		log.Fatal(err)
	}
}
