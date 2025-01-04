package main

import (
	"context"
	"net/http"
	"src/api"
	"src/cache"
	config "src/env"
	"src/router"
)

func main() {
	logger := api.Logger_init()
	if err := config.LoadEnv(logger); err != nil {
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

	r := router.SetupRouter(redisClient, param, logger)

	port := config.GetPort()
	logger.Infof("Starting server on port: %s", port)
	http.ListenAndServe(":"+port, r)
}
