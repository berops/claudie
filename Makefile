.PHONY: gen contentbox scheduler builder terraformer wireguardian kubeEleven

#Generate all .proto files
gen:
	protoc --proto_path=proto proto/*.proto --go_out=plugins=grpc:.

contextbox:
	go run services/context-box/server/server.go

scheduler:
	go run services/scheduler/scheduler.go

builder:
	go run services/builder/builder.go

terraformer:
	go run services/terraformer/server/server.go services/terraformer/server/terraform.go

wireguardian:
	go run services/wireguardian/server/server.go

kubeEleven:
	go run services/kube-eleven/server/server.go

test:
	go test ./testing-framework/...