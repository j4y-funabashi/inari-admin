test:
	go test -v ./...

up:
	docker-compose build
	docker-compose down
	docker-compose up
