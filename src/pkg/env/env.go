package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func LoadEnv(logger *logrus.Logger, path_toenv string) error {
	if err := godotenv.Load(path_toenv); err != nil {
		logger.Fatalf("Error loading .env file: %v", err)
		return err
	}
	return nil
}

func GetAPIKey(logger *logrus.Logger) string {
	key := os.Getenv("API_KEY")
	if key == "" {
		log.Fatal("API_KEY environment variable is not set")
	}
	return key
}

func GetPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified in env vars
	}
	return port
}
