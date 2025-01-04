package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"src/api"
	"src/cache"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"github.com/redis/go-redis/v9"
)

func main() {
	logger := api.Logger_init()
	err := godotenv.Load()
	if err != nil {
		logger.Fatal("Error loading .env file")
	}
	key := os.Getenv("API_KEY")
	if key == "" {
		log.Fatal("API_KEY environment variable is not set")
	}
	var param api.Parameters = api.Parameters{Sity_code: "", Key: key, RedisKey: ""}

	defer api.Logger_close(logger)

	ctx := context.Background()

	redisClient, err := cache.NewRedisClient(ctx, "redis.yaml", logger)
	if err != nil {
		logger.Fatalf("Failed to create redis client: %v", err)
	}

	defer redisClient.Close()

	r := mux.NewRouter()
	r.HandleFunc("/weather/{city}", func(w http.ResponseWriter, r *http.Request) {
		handleWeatherRequest(w, r, redisClient, param, logger)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified in env vars
	}

	logger.Infof("Starting server on port: %s", port)
	http.ListenAndServe(":"+port, r)

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

	ttl, err := redisClient.PTTL(ctx, key).Result()
	if err != nil {
		return nil, false, fmt.Errorf("error while getting TTL of key: %w", err)
	}

	oneHourAgo := time.Now().Add(-time.Hour)

	keyExpirationTime := time.Now().Add(ttl)

	if keyExpirationTime.After(oneHourAgo) {
		return []byte(val), true, nil
	}

	return nil, false, nil
}

// handleWeatherRequest processes weather requests for a specific city
func handleWeatherRequest(w http.ResponseWriter, r *http.Request, redisClient *redis.Client, param api.Parameters, logger *logrus.Logger) {

	vars := mux.Vars(r)
	city := vars["city"]

	if city == "" {
		http.Error(w, "City is required", http.StatusBadRequest)
		return
	}
	param.Sity_code = city
	param.RedisKey = fmt.Sprintf("weather_%s", city)

	data, err := GetWeather(context.Background(), redisClient, param, api.Init_url(param, logger), logger)
	if err != nil {
		logger.Printf("Failed to get weather data: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		bytes.NewBufferString(`{"error": "Failed to get weather data"}`).WriteTo(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

//GetWeather: Gets the weather from cache or external api
func GetWeather(ctx context.Context, redisClient *redis.Client, param api.Parameters, url string, l *logrus.Logger) ([]byte, error) {
	var v []byte
	v, isValid, err := checkCache(ctx, redisClient, param.RedisKey, param.Key, l)
	if isValid {
		l.Printf("%s-old value", v)
		l.Print("old value success get")
	} else if err == nil {
		v, err = cache.Set_weather_in_redis(ctx, redisClient, param.RedisKey, url, l)
		if err != nil {
			l.Fatalf("error in get weather - %v", err)
		} else {
			l.Printf("%s-new value", v)
			l.Print("new value success get")
		}
	} else {
		return nil, err
	}
	return v, nil
}
