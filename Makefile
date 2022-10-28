.PHONY: gen contextbox scheduler builder terraformer ansibler kubeEleven test dockerUp dockerDown dockerBuild database minio

#Generate all .proto files
gen:
	protoc  --go-grpc_out=. --go_out=. proto/*.proto

contextbox:
	go run ./services/context-box/server

scheduler:
	go run ./services/scheduler

builder:
	go run ./services/builder

terraformer:
	go run services/terraformer/server/server.go 

ansibler:
	go run ./services/ansibler/server

kubeEleven:
	go run services/kube-eleven/server/server.go

kuber:
	go run services/kuber/server/server.go

frontend:
	go run ./services/frontend

database:
	docker run --rm -p 27017:27017 -v ~/mongo/data:/data/db mongo:5

minio:
# mkdir will simulate the automatic bucket creation 
	mkdir -p ~/minio/data/claudie-tf-state-files
	docker run --rm -p 9000:9000 -p 9001:9001 --name minio -v ~/minio/data:/data quay.io/minio/minio server /data --console-address ":9001"

dynamodb:
	docker run --rm -p 8000:8000 --name dynamodb-local -v ~/dynamodb:/home/dynamodblocal/data amazon/dynamodb-local:latest -jar DynamoDBLocal.jar -sharedDb -dbPath ./data

dynamodb-create-table:
	aws dynamodb create-table --attribute-definitions AttributeName=LockID,AttributeType=S --table-name claudie --key-schema AttributeName=LockID,KeyType=HASH --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1  --output json --endpoint-url http://localhost:8000 --no-cli-pager

dynamodb-scan-table:
	aws dynamodb scan --table-name claudie --endpoint-url http://localhost:8000 --no-cli-pager

# -timeout 0 will disable default timeout
test:
	go test -v ./services/testing-framework/... -timeout 0 -count=1

# Run all services in docker containers via docker-compose on a local machine
dockerUp:
	docker compose --env-file ./manifests/claudie/.env up

dockerDown:
	docker compose --env-file ./manifests/claudie/.env down

dockerBuild:
	docker compose --env-file ./manifests/claudie/.env build 

lint:
	golangci-lint run