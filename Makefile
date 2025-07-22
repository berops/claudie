.PHONY: proto manager builder terraformer ansibler kubeEleven test database minio containerimgs crd crd-apply controller-gen kind-load-images

# Enforce same version of protoc
PROTOC_VERSION = "29.3"
CURRENT_VERSION = $$(protoc --version | awk '{print $$2}')
# Generate all .proto files
proto:
	@if [ "$(CURRENT_VERSION)" = "$(PROTOC_VERSION)" ]; then \
		protoc --proto_path=proto --go_out=paths=source_relative:proto/pb --go-grpc_out=paths=source_relative:proto/pb proto/*.proto proto/spec/*.proto ;\
	else \
		echo "Please update your protoc version. Current $(CURRENT_VERSION) | Required $(PROTOC_VERSION)"; \
	fi

# Start manager on a local environment, exposted on port 50055
manager:
	GOLANG_LOG=debug PROMETHEUS_PORT=9091 go run ./services/manager/cmd/api-server
# Start Builder service on a local environment
builder:
	GOLANG_LOG=debug PROMETHEUS_PORT=9092 go run ./services/builder
# Start Terraformer service on a local environment, exposed on port 50052
terraformer:
	GOLANG_LOG=debug BUCKET_URL="http://localhost:9000" DYNAMO_URL="http://localhost:8000" AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin DYNAMO_TABLE_NAME=claudie PROMETHEUS_PORT=9093 go run ./services/terraformer/server

# Start Ansibler service on a local environment, exposed on port 50053
ansibler:
	GOLANG_LOG=debug PROMETHEUS_PORT=9094 go run ./services/ansibler/server

# Start Kube-eleven service on a local environment, exposed on port 50054
kube-eleven:
	GOLANG_LOG=debug PROMETHEUS_PORT=9095 go run ./services/kube-eleven/server

# Start Kuber service on a local environment, exposed on port 50057
kuber:
	GOLANG_LOG=debug PROMETHEUS_PORT=9096 go run ./services/kuber/server

# Start Claudie-operator service on a local environment
# This is not necessary to have running on local environtment, to inject input manifest,
# use API directly from either /services/manager
operator:
	GOLANG_LOG=debug go run ./services/claudie-operator

# Start the database for configs, containing input manifests
mongo:
	mkdir -p ~/mongo/data
	docker run --name mongo -d --rm -p 27017:27017 -v ~/mongo/data:/data/db mongo:5

# Start minio backend for state files used in terraform
minio:
# mkdir will simulate the automatic bucket creation
	mkdir -p ~/minio/data/claudie-tf-state-files
	docker run --name minio -d --rm -p 9000:9000 -p 9001:9001 --name minio -v ~/minio/data:/data quay.io/minio/minio server /data --console-address ":9001"

# Start DynamoDB backend used for state file locks
dynamodb:
	mkdir -p ~/dynamodb
	docker run --name dynamodb -d --rm -p 8000:8000 -v ~/dynamodb:/home/dynamodblocal/data amazon/dynamodb-local:1.21.0 -jar DynamoDBLocal.jar -sharedDb -dbPath ./data

# Start Testing-framework, which will inject manifests from /services/testing-framework/test-sets
# -timeout 0 will disable default timeout
# Successful test will end with infrastructure being destroyed
test:
	AUTO_CLEAN_UP=TRUE GOLANG_LOG=debug go test -v ./services/testing-framework -timeout 0 -count=1 -run TestClaudie

# Run the golang linter
lint:
	golangci-lint run

# Start all data stores at once,in docker containers, to simplify the local development
datastoreStart: mongo minio dynamodb

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
	sed -i "s/image: ghcr.io\/berops\/claudie\/autoscaler-adapter/&:$(REV)/" services/kuber/templates/cluster-autoscaler.goyaml
	for service in $(SERVICES) ; do \
		echo " --- building $$service --- "; \
		DOCKER_BUILDKIT=1 docker build --build-arg=TARGETARCH="$(TARGETARCH)" -t "ghcr.io/berops/claudie/$$service:$(REV)" -f ./services/$$service/Dockerfile . ; \
	done
	sed -i "s/adapter:.*$$/adapter/" services/kuber/templates/cluster-autoscaler.goyaml

kind-load-images:
	for service in $(SERVICES) ; do \
		echo " --- loading $$service to kind cluster --- "; \
		kind load docker-image ghcr.io/berops/claudie/$$service:$(REV); \
	done

# Generate CustomResourceDefinition objects.
crd:
	go tool controller-gen rbac:roleName=manager-role crd paths="./..." output:crd:artifacts:config=manifests/claudie/crd

crd-apply:
	go tool controller-gen crd paths="./..." output:crd:artifacts:config=manifests/claudie/crd && kubectl apply -k ./manifests/claudie/crd/
