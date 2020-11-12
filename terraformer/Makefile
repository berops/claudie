.PHONY: gen server client

gen:
	protoc terraformerpb/terraformer.proto --go_out=plugins=grpc:.

server:
	go run ./server

client:
	go run ./client