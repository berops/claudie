.PHONY: gen contentbox scheduler builder terraformer wireguardian kubeEleven docker

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

docker:
	DOCKER_BUILDKIT=1 docker-compose --env-file ./K8s-dev-cluster/.env up -d

# -timeout 0 will disable default timeout
test:
	go test -v ./services/testing-framework/... -timeout 0
