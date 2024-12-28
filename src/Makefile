#  sudo apt install redis-server
all: build

run:
	go run main.go

redis_create:
	sudo systemctl start redis
	sudo systemctl status redis
	redis-cli ping
