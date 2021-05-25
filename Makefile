.PHONY: gen contentbox scheduler builder

#Generate all .proto files
gen:
	protoc --proto_path=proto proto/*.proto --go_out=plugins=grpc:.

contentbox:
	go run services/context-box/server/server.go

scheduler:
	go run services/scheduler/scheduler.go

builder:
	go run services/builder/builder.go