run-plugin: 
	go build -o sessionPlugin .;sudo APISIX_LISTEN_ADDRESS=unix:/tmp/runner.sock ./sessionPlugin 

build-dummy-server:
	cd dummyserver/cmd; go build main.go; docker build -t revoly/dummyhttp .

start-apisix: #For local tests
	cd ../testingplugin; docker-compose up