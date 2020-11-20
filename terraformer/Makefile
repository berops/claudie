.PHONY: gen server client destroy

gen:
	protoc terraformerpb/terraformer.proto --go_out=plugins=grpc:.

server:
	go run ./server

client:
	go run ./client

destroy:
	terraform destroy --auto-approve -state=terraform/terraform.tfstate terraform 