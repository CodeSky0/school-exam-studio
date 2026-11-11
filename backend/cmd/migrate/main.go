package main

import (
	"log"
	"os"

	"school-exam-studio/backend/internal/platform/migrator"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	if err := migrator.Up(databaseURL); err != nil {
		log.Fatal(err)
	}
}
