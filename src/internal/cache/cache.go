package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
	"weather-API/internal/api"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/redis/go-redis/v9"
)

var cacheTTL = 1 * time.Hour

// storage/redis.go
type RedisConfig struct {
	Addr        string        `yaml:"addr"`
	Password    string        `yaml:"password"`
	User        string        `yaml:"user"`
	DB          int           `yaml:"db"`
	MaxRetries  int           `yaml:"max_retries"`
	DialTimeout time.Duration `yaml:"dial_timeout"`
	Timeout     time.Duration `yaml:"timeout"`
}

func LoadRedisConfig(filename string) (RedisConfig, error) {
	var config RedisConfig
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return config, fmt.Errorf("failed to read file %v: %w", filename, err)
	}
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		return config, fmt.Errorf("failed to unmarshal yaml file %v: %w", filename, err)
	}
	return config, nil
}

func GetWeather(ctx context.Context, redisClient *redis.Client, param api.Parameters, url string, l *logrus.Logger) ([]byte, error) {
	var v []byte
	v, isValid, err := CheckCache(ctx, redisClient, param.RedisKey, param.Key, l)
	if isValid {
		l.Printf("%s-old value", v)
		l.Print("old value success get")
	} else if err == nil {
		v, err = Set_weather_in_redis(ctx, redisClient, param.RedisKey, url, l)
		if err != nil {
			l.Printf("error in get weather - %v", err)
			return nil, err
		} else {
			l.Printf("%s-new value", v)
			l.Print("new value success get")
		}
	} else {
		return nil, err
	}
	return v, nil
}

// storage/redis.go
func NewRedisClient(ctx context.Context, config_path string, logger *logrus.Logger) (*redis.Client, error) {
	cfg, err := LoadRedisConfig(config_path)
	if err != nil {
		logger.Fatalf("Failed to load redis config: %v", err)
		return nil, err
	}

	db := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		Username:     cfg.User,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
	})
	err = db.Ping(ctx).Err()
	if err != nil {
		logger.Errorf("Failed to connect to redis %v", err)
		return nil, err
	}
	logger.Info("Connected to Redis")
	return db, nil
}

// storage/redis.go
func NewClient(ctx context.Context, cfg RedisConfig) (*redis.Client, error) {
	db := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		Username:     cfg.User,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.Timeout,
		WriteTimeout: cfg.Timeout,
	})

	if err := db.Ping(ctx).Err(); err != nil {
		fmt.Printf("failed to connect to redis server: %s\n", err.Error())
		return nil, err
	}

	return db, nil
}

func Set_weather_in_redis(ctx context.Context, redisClient *redis.Client, redisKey string, url string, logger *logrus.Logger) ([]byte, error) {

	err, weather := api.Get_weather_info(url, logger)
	if err != nil {
		logger.Printf("failed to get weather %v", err)
		return nil, err
	}

	jsonData, err := json.Marshal(weather)
	if err != nil {
		logger.Printf("failed to marshal weather to json %v", err)
		return nil, err
	}

	err = redisClient.Set(ctx, redisKey, jsonData, cacheTTL).Err()
	if err != nil {
		logger.Printf("failed to set value in redis: %v", err)
		return nil, err
	}

	fmt.Printf("Value successfully saved in Redis under key: %s\n", redisKey)
	return jsonData, nil
}

func CheckCache(ctx context.Context, redisClient *redis.Client, redisKey string, key string, l *logrus.Logger) ([]byte, bool, error) {

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

// HandleWeatherRequest processes weather requests for a specific city
func HandleWeatherRequest(w http.ResponseWriter, r *http.Request, redisClient *redis.Client, param api.Parameters, logger *logrus.Logger) {

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
