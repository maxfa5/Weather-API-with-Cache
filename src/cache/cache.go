package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"src/api"
	"time"

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

	weather := api.Get_weather_info(url, logger)
	jsonData, err := json.Marshal(weather)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal weather to json %w", err)
	}

	err = redisClient.Set(ctx, redisKey, jsonData, cacheTTL).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to set value in redis: %w", err)
	}

	fmt.Printf("Value successfully saved in Redis under key: %s\n", redisKey)
	return jsonData, nil
}
