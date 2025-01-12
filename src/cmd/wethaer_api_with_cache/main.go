package main

import (
	"context"
	"net/http"
	"weather-API/internal/api"
	"weather-API/internal/cache"
	config "weather-API/pkg/env"
	"weather-API/pkg/router"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

func SetupRouter(redisClient *redis.Client, param api.Parameters, logger *logrus.Logger) *mux.Router {
	r := mux.NewRouter()
	r.Use(router.RateLimitingMiddleware(logger))

	r.HandleFunc("/weather/{city}", func(w http.ResponseWriter, r *http.Request) {
		cache.HandleWeatherRequest(w, r, redisClient, param, logger)
	})

	return r
}

func main() {
	logger := api.Logger_init()
	if err := config.LoadEnv(logger, "./cmd/wethaer_api_with_cache/.env"); err != nil {
		logger.Fatal(err)
	}
	key := config.GetAPIKey(logger)

	var param api.Parameters = api.Parameters{Sity_code: "", Key: key, RedisKey: ""}

	defer api.Logger_close(logger)

	ctx := context.Background()

	redisClient, err := cache.NewRedisClient(ctx, "redis.yaml", logger)
	if err != nil {
		logger.Fatalf("Failed to create redis client: %v", err)
	}

	defer redisClient.Close()

	r := SetupRouter(redisClient, param, logger)

	port := config.GetPort()
	logger.Infof("Starting server on port: %s", port)
	http.ListenAndServe(":"+port, r)
}
