#  sudo apt install redis-server
# or
# sudo service redis-server start
all: build

run:
	go run ./cmd/wethaer_api_with_cache/main.go

redis_create:
	sudo systemctl start redis
	sudo systemctl status redis
	redis-cli ping
