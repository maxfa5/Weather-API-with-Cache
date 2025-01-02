package main

import (
	"context"
	"encoding/json"
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
	var param api.Parameters = api.Parameters{Sity_code: sity_code, Key: key, RedisKey: (fmt.Sprintf("weather_%s", sity_code))}

	logger := api.Logger_init()
	defer api.Logger_close(logger)

	ctx := context.Background()

	redisClient, err := cache.NewRedisClient(ctx, "redis.yaml", logger)
	if err != nil {
		logger.Fatalf("Failed to create redis client: %v", err)
	}

	defer redisClient.Close()
	GetWeather(ctx, redisClient, param, api.Init_url(param, logger), logger)
}

func checkCache(ctx context.Context, redisClient *redis.Client, redisKey string, key string, l *logrus.Logger) ([]byte, bool, error) {

	val, err := redisClient.Get(ctx, redisKey).Result()
	if err == redis.Nil {
		l.Print("Redis in null")
		return nil, false, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("error while getting from cache: %w", err)
	}

	// 2. Get remaining time to live in milliseconds
	ttl, err := redisClient.PTTL(ctx, key).Result()
	if err != nil {
		return nil, false, fmt.Errorf("error while getting TTL of key: %w", err)
	}

	// 3. Check time difference
	oneHourAgo := time.Now().Add(-time.Hour)

	// calculate the key's expiration time by adding the remaining ttl to current time.
	keyExpirationTime := time.Now().Add(ttl)

	// compare the expiration time with one hour ago to see if the data is fresh.
	if keyExpirationTime.After(oneHourAgo) {
		jsonRes, err := json.Marshal(val)
		if err != nil {
			return nil, false, fmt.Errorf("failed to marshal weather to json %w", err)
		}
		return jsonRes, true, nil
	}

	return nil, false, nil
}

func GetWeather(ctx context.Context, redisClient *redis.Client, param api.Parameters, url string, l *logrus.Logger) {
	var v []byte
	v, isValid, err := checkCache(ctx, redisClient, param.RedisKey, param.Key, l)
	if isValid {
		fmt.Printf("%s-old value", v)
		l.Print("old value success get")
	} else if err == nil {
		v, err = cache.Set_weather_in_redis(ctx, redisClient, param.RedisKey, url, l)
		if err != nil {
			l.Fatalf("error in get weather - %v", err)
		} else {
			fmt.Printf("%s-new value", v)
			l.Print("new value success get")
		}
	}
}
