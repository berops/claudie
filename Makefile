.PHONY: gen

#Generate all .proto files
gen:
	protoc --proto_path=proto proto/*.proto --go_out=plugins=grpc:.