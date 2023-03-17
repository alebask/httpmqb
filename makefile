.PHONY: run
run:
	go run cmd/main.go

.PHONY: build
build:
	go build -o httpmqb cmd/main.go

.PHONY: docker
docker: 
	docker-compose  build 

.PHONY: dockerup
dockerup: 
	docker-compose  up

.PHONY: dockerdown
dockerdown: 
	docker-compose  down

.PHONY: dockerdebug
dockerdebug: 
	docker-compose  -f docker-compose-debug.yml up --build
