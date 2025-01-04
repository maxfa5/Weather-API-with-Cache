package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"src/api"
	"src/cache"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"github.com/redis/go-redis/v9"
)

// Create a struct to hold each client's rate limiter
type Client struct {
	limiter *rate.Limiter
}

// In-memory storage for clients
var clients = make(map[string]*Client)
var mu sync.Mutex
var COUNT_RESP_PRE_MIN = 5

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
	r.Use(rateLimitingMiddleware(logger))

	r.HandleFunc("/weather/{city}", func(w http.ResponseWriter, r *http.Request) {
		handleWeatherRequest(w, r, redisClient, param, logger)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified in env vars
	}

	logger.Infof("Starting server on port: %s", port)
	http.ListenAndServe(":"+port, r)
}

func rateLimitingMiddleware(logger *logrus.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			limiter := getClientLimiter(ip, logger)

			if !limiter.Allow() {
				logger.Warnf("Rate limit exceeded for ip: %s", ip)
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
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

func getClientIP(r *http.Request) string {
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		return strings.Split(forwardedFor, ",")[0]
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

//getClientLimiter: Get a client's rate limiter or create one if it doesn't exist
func getClientLimiter(ip string, logger *logrus.Logger) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	// If the client already exists, return the existing limiter
	if client, exists := clients[ip]; exists {
		return client.limiter
	}

	// Create a new limiter with 5 requests per minute
	rateLimit := rate.Every(time.Minute)
	limiter := rate.NewLimiter(rateLimit, COUNT_RESP_PRE_MIN)
	logger.Printf("Creating a new limiter for ip: %s, with rateLimit: %v", ip, rateLimit)
	clients[ip] = &Client{limiter: limiter}
	return limiter
}
