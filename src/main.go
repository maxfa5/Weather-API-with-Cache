package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/redis/go-redis/v9"
)

var cacheTTL = 72 * time.Hour

type Hour struct {
	Datetime string `json:"datetime"`
}
type Day struct {
	Datetime  string  `json:"datetime"`
	Temp      float64 `json:"temp"`
	Moonphase float64 `json:"moonphase"`
	Hours     []Hour  `json:"hours"`
}
type WeatherData struct {
	Address     string `json:"address"`
	Description string `json:"description"`
	Days        []Day  `json:"days"`
}

type Weather struct {
	City        string
	Temperature string
	Forecast    string
	Date        string
	Moonphase   float64
}
type Parameters struct {
	sity_code string
	key       string
}

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

func main() {
	sity_code := "surgut"
	key := "6TA3N5JVQTPB4ATZCKHL238BH"
	var param Parameters = Parameters{sity_code, key}

	logger := logger_init()
	defer logger_close(logger)

	ctx := context.Background()

	redisConfig, err := loadRedisConfig("redis.yaml")
	if err != nil {
		logger.Fatalf("Failed to load redis config: %v", err)
	}

	redisClient, err := NewRedisClient(ctx, redisConfig, logger)
	if err != nil {
		logger.Fatalf("Failed to create redis client: %v", err)
	}
	defer redisClient.Close()

	url := init_url(param, logger)
	set_weather_in_redis(ctx, redisClient, url, logger)
}

func loadRedisConfig(filename string) (RedisConfig, error) {
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
func NewRedisClient(ctx context.Context, cfg RedisConfig, logger *logrus.Logger) (*redis.Client, error) {
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
	err := db.Ping(ctx).Err()
	if err != nil {
		logger.Errorf("Failed to connect to redis %v", err)
		return nil, err
	}
	logger.Info("Connected to Redis")
	return db, nil
}

func logger_close(logger *logrus.Logger) {
	if logFile, ok := logger.Out.(*os.File); ok && logFile != os.Stdout {
		logFile.Close()
	}
}
func logger_init() (l *logrus.Logger) {
	l = logrus.New()
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Failed to open log file:", err)
		l.Out = os.Stdout
	} else {
		l.Out = file
	}
	return l
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
		// g.()
		return nil, err
	}

	return db, nil
}

func init_url(param Parameters, l *logrus.Logger) (url string) {
	url = fmt.Sprintf("https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/%s/today?unitGroup=metric&key=%s&contentType=json", param.sity_code, param.key)
	l.Print("Success init URL")
	return url
}

func get_data(allData *WeatherData) error {
	if len(allData.Days) == 0 {
		return fmt.Errorf("no data available")
	}

	new_stat := Weather{}
	new_stat.City = allData.Address
	new_stat.Temperature = fmt.Sprintf("%.2f", allData.Days[0].Temp)
	new_stat.Date = allData.Days[0].Datetime
	new_stat.Moonphase = allData.Days[0].Moonphase
	new_stat.Forecast = allData.Description

	return nil
}

func get_weather_info(url string, l *logrus.Logger) (new_stat Weather) {
	resp, err := http.Get(url)
	if err != nil {
		l.Panic("can`t get JSON")
		panic(err)
	}
	defer resp.Body.Close()

	resp_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		l.Panicf("can`t get JSON, %v", err)
		panic(err)
	}

	var weatherData WeatherData
	err = json.Unmarshal(resp_body, &weatherData)
	if err != nil {
		l.Panicf("can`t get JSON, %v", err)
		panic(err)
	}

	if err := get_data(&weatherData); err != nil {
		l.Panicf("error processing data: %v", err)
		panic(err)
	}
	l.Print("Success get JSON")

	return Weather{
		City:        weatherData.Address,
		Temperature: fmt.Sprintf("%.2f", weatherData.Days[0].Temp),
		Date:        weatherData.Days[0].Datetime,
		Moonphase:   weatherData.Days[0].Moonphase,
		Forecast:    weatherData.Description,
	}
}

func set_weather_in_redis(ctx context.Context, redisClient *redis.Client, url string, logger *logrus.Logger) error {
	cacheKey := fmt.Sprintf("weather:%s", url)

	weather := get_weather_info(url, logger)
	jsonData, err := json.Marshal(weather)
	if err != nil {
		return fmt.Errorf("failed to marshal weather to json %w", err)
	}

	err = redisClient.Set(ctx, cacheKey, jsonData, cacheTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to set value in redis: %w", err)
	}

	fmt.Printf("Value successfully saved in Redis under key: %s\n", cacheKey)
	return nil
}
