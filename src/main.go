package main

import (
	"context"
	"fmt"
	"src/api"
	"src/cache"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/redis/go-redis/v9"
)

func main() {
	sity_code := "surgut"
	key := "6TA3N5JVQTPB4ATZCKHL238BH"
	var param api.Parameters = api.Parameters{Sity_code: sity_code, Key: key}

	logger := api.Logger_init()
	defer api.Logger_close(logger)

	ctx := context.Background()

	redisClient, err := cache.NewRedisClient(ctx, "redis.yaml", logger)
	if err != nil {
		logger.Fatalf("Failed to create redis client: %v", err)
	}

	defer redisClient.Close()
	v, isValid, err := checkCache(ctx, redisClient, sity_code, logger)
	if err != nil {
		logger.Fatalf("Failed to check cache: %v", err)
	}
	if !isValid {

		logger.Print("cache is expired or null!")
		url := api.Init_url(param, logger)
		value, err := cache.Set_weather_in_redis(ctx, redisClient, sity_code, url, logger)
		if err != nil {
			logger.Errorf("Failed to set data to cache %v", err)
		}
		fmt.Printf("%s-new value", value)
	} else {
		logger.Print("cache is ok!")
		fmt.Printf("%s-old value", v)
	}
}

func checkCache(ctx context.Context, redisClient *redis.Client, key string, l *logrus.Logger) (string, bool, error) {

	val, err := redisClient.Get(ctx, fmt.Sprintf("weather_%s", key)).Result()
	if err == redis.Nil {
		l.Print("Redis in null")
		return "", false, nil
	} else if err != nil {
		return "", false, fmt.Errorf("error while getting from cache: %w", err)
	}

	// 2. Get remaining time to live in milliseconds
	ttl, err := redisClient.PTTL(ctx, key).Result()
	if err != nil {
		return "", false, fmt.Errorf("error while getting TTL of key: %w", err)
	}

	// 3. Check time difference
	oneHourAgo := time.Now().Add(-time.Hour)

	// calculate the key's expiration time by adding the remaining ttl to current time.
	keyExpirationTime := time.Now().Add(ttl)

	// compare the expiration time with one hour ago to see if the data is fresh.
	if keyExpirationTime.After(oneHourAgo) {
		return val, true, nil
	}

	return "", false, nil
}
