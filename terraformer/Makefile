.PHONY: gen server client destroy

gen:
	protoc terraformerpb/terraformer.proto --go_out=plugins=grpc:.

server:
	go run ./server

client:
	go run ./client

destroy:
	terraform destroy --auto-approve terraform && rm -r terraform && rm -r .terraform