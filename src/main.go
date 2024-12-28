package main

import (
	"context"

	"src/api"
)

func main() {
	sity_code := "surgut"
	key := "6TA3N5JVQTPB4ATZCKHL238BH"
	var param api.Parameters = api.Parameters{sity_code, key}

	logger := api.Logger_init()
	defer api.Logger_close(logger)

	ctx := context.Background()

	redisClient, err := api.NewRedisClient(ctx, "redis.yaml", logger)
	if err != nil {
		logger.Fatalf("Failed to create redis client: %v", err)
	}

	defer redisClient.Close()

	url := api.Init_url(param, logger)
	err = api.Set_weather_in_redis(ctx, redisClient, sity_code, url, logger)
	if err != nil {
		logger.Errorf("Failed to set data to cache %v", err)
		// return err
	}
}
