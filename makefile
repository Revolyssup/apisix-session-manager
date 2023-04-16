run-plugin: 
	go build -o sessionPlugin .;sudo APISIX_LISTEN_ADDRESS=unix:/tmp/runner.sock ./sessionPlugin 

start-apisix: #For local tests
	docker-compose down; docker-compose up