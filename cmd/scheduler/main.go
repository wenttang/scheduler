package main

import (
	"log"
	"net/http"
	"os"

	daprd "github.com/dapr/go-sdk/service/http"
	"github.com/wenttang/scheduler/internal/scheduler"
)

func main() {
	Main()
}

func Main() {
	const defaultPort = ":8000"
	port := os.Getenv("SCHEDULER_PORT")
	if port == "" {
		port = defaultPort
	}
	s := daprd.NewService(port)
	s.RegisterActorImplFactory(scheduler.New)

	if err := s.Start(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("error listenning: %v", err)
	}
}
