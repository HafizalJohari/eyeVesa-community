package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/hafizaljohari/eyeVesa/gateway/control-plane/cmd/api/handlers"
)

func main() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Route("/v1", func(r chi.Router) {
		r.Post("/agents/register", handlers.RegisterAgent)
		r.Get("/agents/{agentID}", handlers.GetAgent)

		r.Post("/resources/register", handlers.RegisterResource)
		r.Get("/resources/{resourceID}", handlers.GetResource)

		r.Post("/mcp", handlers.HandleMCP)
	})

	addr := ":8080"
	log.Printf("AgentID Control Plane starting on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}