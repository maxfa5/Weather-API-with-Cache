package router

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"src/api"
	"src/cache"
	"time"

	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

// Create a struct to hold each client's rate limiter
type Client struct {
	limiter *rate.Limiter
}

// In-memory storage for clients
var clients = make(map[string]*Client)
var mu sync.Mutex
var COUNT_REQ_PRE_MIN = 5

func SetupRouter(redisClient *redis.Client, param api.Parameters, logger *logrus.Logger) *mux.Router {
	r := mux.NewRouter()
	r.Use(RateLimitingMiddleware(logger))

	r.HandleFunc("/weather/{city}", func(w http.ResponseWriter, r *http.Request) {
		handleWeatherRequest(w, r, redisClient, param, logger)
	})

	return r
}
func RateLimitingMiddleware(logger *logrus.Logger) func(http.Handler) http.Handler {
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
	limiter := rate.NewLimiter(rateLimit, COUNT_REQ_PRE_MIN)
	logger.Printf("Creating a new limiter for ip: %s, with rateLimit: %v", ip, rateLimit)
	clients[ip] = &Client{limiter: limiter}
	return limiter
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

	data, err := cache.GetWeather(context.Background(), redisClient, param, api.Init_url(param, logger), logger)
	if err != nil {
		logger.Errorf("Failed to get weather data: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		response := `{"error": "Failed to get weather data"}`
		w.Write([]byte(response))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		logger.Errorf("Failed to write response: %v", err)
	}
}
