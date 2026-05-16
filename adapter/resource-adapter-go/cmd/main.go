package main

import (
	"context"
	"log"
	"os"

	"github.com/hafizaljohari/eyeVesa/adapter/resource-adapter-go/cmd/server"
)

func main() {
	resourceName := os.Getenv("RESOURCE_NAME")
	if resourceName == "" {
		resourceName = "unnamed-resource"
	}

	endpoint := os.Getenv("GATEWAY_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9443"
	}

	srv := server.New(resourceName, endpoint)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Resource Adapter '%s' starting, connecting to gateway at %s", resourceName, endpoint)

	if err := srv.Run(ctx); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}