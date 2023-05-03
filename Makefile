.PHONY: proto contextbox scheduler builder terraformer ansibler kubeEleven test database minio containerimgs

# Generate all .proto files
proto:
	protoc  --go-grpc_out=. --go_out=. proto/*.proto

# Start Context-box service on a local environment, exposed on port 50055
contextbox:
	GOLANG_LOG=debug go run ./services/context-box/server

# Start Scheduler service on a local environment
scheduler:
	GOLANG_LOG=debug go run ./services/scheduler

# Start Builder service on a local environment
builder:
	GOLANG_LOG=debug go run ./services/builder
# Start Terraformer service on a local environment, exposed on port 50052
terraformer:
	GOLANG_LOG=debug go run services/terraformer/server/server.go 

# Start Ansibler service on a local environment, exposed on port 50053
ansibler:
	GOLANG_LOG=debug go run ./services/ansibler/server

# Start Kube-eleven service on a local environment, exposed on port 50054
kube-eleven:
	GOLANG_LOG=debug go run services/kube-eleven/server/server.go

# Start Kuber service on a local environment, exposed on port 50057
kuber:
	GOLANG_LOG=debug go run services/kuber/server/server.go

# Start Frontend service on a local environment
# This is not necessary to have running on local environtment, to inject input manifest,
# use API directly from either /services/context-box/client/client_test.go -run TestSaveConfigFrontEnd,
# or use testing-framework
frontend:
	GOLANG_LOG=debug go run ./services/frontend

# Start the database for configs, containing input manifests
database:
	docker run --rm -p 27017:27017 -v ~/mongo/data:/data/db mongo:5

# Start minio backend for state files used in terraform
minio:
# mkdir will simulate the automatic bucket creation 
	mkdir -p ~/minio/data/claudie-tf-state-files
	docker run --rm -p 9000:9000 -p 9001:9001 --name minio -v ~/minio/data:/data quay.io/minio/minio server /data --console-address ":9001"

# Start DynamoDB backend used for state file locks
dynamodb:
	docker run --rm -p 8000:8000 --name dynamodb-local -v ~/dynamodb:/home/dynamodblocal/data amazon/dynamodb-local:1.21.0 -jar DynamoDBLocal.jar -sharedDb -dbPath ./data

# Start Testing-framework, which will inject manifests from /services/testing-framework/test-sets
# -timeout 0 will disable default timeout
# Successful test will end with infrastructure being destroyed
test:
	AUTO_CLEAN_UP=TRUE go test -v ./services/testing-framework -timeout 0 -count=1 -run TestClaudie

# Run the golang linter
lint:
	golangci-lint run

# Start all data stores at once,in docker containers, to simplify the local development
datastoreStart:
	docker run --rm -d -p 27017:27017 --name mongo -v ~/mongo/data:/data/db mongo:5
	docker run --rm -d -p 9000:9000 -p 9001:9001 --name minio -v ~/minio/data:/data quay.io/minio/minio server /data --console-address ":9001"
	docker run --rm -d -p 8000:8000 --name dynamodb -v ~/dynamodb:/home/dynamodblocal/data amazon/dynamodb-local:1.21.0 -jar DynamoDBLocal.jar -sharedDb -dbPath ./data

# Stops all data stores at once, which will also remove docker containers
datastoreStop:
	docker stop mongo
	docker stop minio
	docker stop dynamodb

# DynamoDB utilities 
dynamodb-create-table:
	aws dynamodb create-table --attribute-definitions AttributeName=LockID,AttributeType=S --table-name claudie --key-schema AttributeName=LockID,KeyType=HASH --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1  --output json --endpoint-url http://localhost:8000 --debug --region local

dynamodb-scan-table:
	aws dynamodb scan --table-name claudie --endpoint-url http://localhost:8000 --region local --no-cli-pager --debug

# We need the value of local architecture to pass to docker as a build arg and
# Go already needs to be installed so we make use of it here.
# Use sed to set the image tag for cluster adapter, clean up at the end.
TARGETARCH = $$(go env GOHOSTARCH)
REV = $$(git rev-parse --short HEAD)
SERVICES = $$(command ls services/)
containerimgs:
	sed -i "s/image: ghcr.io\/berops\/claudie\/autoscaler-adapter/&:$(REV)/" services/kuber/templates/autoscaler-adapter.goyaml	
	for service in $(SERVICES) ; do \
		echo " --- building $$service --- "; \
		DOCKER_BUILDKIT=1 docker build --build-arg=TARGETARCH="$(TARGETARCH)" -t "ghcr.io/berops/claudie/$$service:$(REV)" -f ./services/$$service/Dockerfile . ; \
	done
	sed -i "s/adapter:.*$$/adapter/" services/kuber/templates/autoscaler-adapter.goyaml
