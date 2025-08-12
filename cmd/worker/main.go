package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/address-parser/app/config"
)

func main() {
	// Load configuration
	if err := config.Load("config/parser.yaml"); err != nil {
		panic(err)
	}

	log.Println("Starting Address Parser Worker...")
	
	// TODO: Implement worker logic for batch processing
	// For now, just keep the process running
	
	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down worker...")
	
	// Give outstanding tasks a deadline for completion
	time.Sleep(5 * time.Second)
	
	log.Println("Worker exited")
}
