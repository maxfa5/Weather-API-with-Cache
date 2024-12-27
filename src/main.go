package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
	sity_code string
	key       string
}

func main() {
	sity_code := "surgut"
	key := "6TA3N5JVQTPB4ATZCKHL238BH"
	var param Parameters = Parameters{sity_code, key}
	url := init_url(param)
	// Make the GET request
	stat := get_weather_info(url)
	fmt.Println(stat)
}

func init_url(param Parameters) (url string) {
	url = fmt.Sprintf("https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/%s/today?unitGroup=metric&key=%s&contentType=json", param.sity_code, param.key)
	return url
}

func init_data(allData WeatherData) (new_stat Weather) {
	new_stat.City = allData.Address
	if len(allData.Days) > 0 {
		new_stat.Temperature = fmt.Sprintf("%.2f", allData.Days[0].Temp)
		new_stat.Date = allData.Days[0].Datetime
		new_stat.Moonphase = allData.Days[0].Moonphase
		new_stat.Forecast = allData.Description
	}
	return new_stat
}

func get_weather_info(url string) (new_stat Weather) {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	resp_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	var weatherData WeatherData
	err = json.Unmarshal(resp_body, &weatherData)
	if err != nil {
		panic(err)
	}
	return init_data(weatherData)
}
