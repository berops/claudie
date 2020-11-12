.PHONY: gen server client

gen:
	protoc terraformerpb/teraformer.proto --go_out=plugins=grpc:.

server:
	go run ./server

client:
	go run ./client