.PHONY: gen server client

gen:
	protoc wireguardianpb/wireguardian.proto --go_out=plugins=grpc:.

server:
	go run ./server
	
client:
	go run ./client