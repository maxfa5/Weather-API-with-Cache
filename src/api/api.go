package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"net/http"

	"github.com/sirupsen/logrus"
)

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
	RedisKey  string
}

// func GetWeather(sity_code string) error {
// 	// sity_code := "surgut"
// 	key := "6TA3N5JVQTPB4ATZCKHL238BH"
// 	var param Parameters = Parameters{sity_code, key}

// 	logger := Logger_init()
// 	defer Logger_close(logger)

// 	ctx := context.Background()

// 	redisClient, err := NewRedisClient(ctx, "redis.yaml", logger)
// 	if err != nil {
// 		logger.Fatalf("Failed to create redis client: %v", err)
// 	}

// 	defer redisClient.Close()

// 	url := Init_url(param, logger)
// 	err = Set_weather_in_redis(ctx, redisClient, sity_code, url, logger)
// 	if err != nil {
// 		logger.Errorf("Failed to set data to cache %v", err)
// 		return err
// 	}
// 	return nil
// }

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
