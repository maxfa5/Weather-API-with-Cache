#  sudo apt install redis-server
# or
# sudo service redis-server start
all: redis run

run:
	go run ./cmd/wethaer_api_with_cache/main.go

redis:
	docker-compose up -d

redis_check_data:
	docker exec -it redis_for_weather_api redis-cli