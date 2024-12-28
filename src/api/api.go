package api

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

var cacheTTL = 12 * time.Hour

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
	Sity_code string
	Key       string
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

func GetWeather(sity_code string) error {
	// sity_code := "surgut"
	key := "6TA3N5JVQTPB4ATZCKHL238BH"
	var param Parameters = Parameters{sity_code, key}

	logger := Logger_init()
	defer Logger_close(logger)

	ctx := context.Background()

	redisClient, err := NewRedisClient(ctx, "redis.yaml", logger)
	if err != nil {
		logger.Fatalf("Failed to create redis client: %v", err)
	}

	defer redisClient.Close()

	url := Init_url(param, logger)
	err = Set_weather_in_redis(ctx, redisClient, sity_code, url, logger)
	if err != nil {
		logger.Errorf("Failed to set data to cache %v", err)
		return err
	}
	return nil
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

func Logger_close(logger *logrus.Logger) {
	if logFile, ok := logger.Out.(*os.File); ok && logFile != os.Stdout {
		logFile.Close()
	}
}
func Logger_init() (l *logrus.Logger) {
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
		return nil, err
	}

	return db, nil
}

func Init_url(param Parameters, l *logrus.Logger) (url string) {
	url = fmt.Sprintf("https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/%s/today?unitGroup=metric&key=%s&contentType=json", param.Sity_code, param.Key)
	l.Print("Success init URL")
	return url
}

func Get_data(allData *WeatherData) error {
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

func Get_weather_info(url string, l *logrus.Logger) (new_stat Weather) {
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

	if err := Get_data(&weatherData); err != nil {
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

func Set_weather_in_redis(ctx context.Context, redisClient *redis.Client, sity_code string, url string, logger *logrus.Logger) error {
	cacheKey := fmt.Sprintf("weather_%s", sity_code)

	weather := Get_weather_info(url, logger)
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
