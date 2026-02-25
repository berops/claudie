.PHONY: proto manager terraformer ansibler kubeEleven test database minio containerimgs crd crd-apply controller-gen kind-load-images nats  kind-deploy

# Enforce same version of protoc
PROTOC_VERSION = "33.4"
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

# Start Terraformer service on a local environment, exposed on port 50052
terraformer:
	GOLANG_LOG=debug BUCKET_URL="http://localhost:9000" AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin PROMETHEUS_PORT=9093 go run ./services/terraformer/cmd/worker

# Start Ansibler service on a local environment, exposed on port 50053
ansibler:
	GOLANG_LOG=debug PROMETHEUS_PORT=9094 go run ./services/ansibler/cmd/worker

# Start Kube-eleven service on a local environment, exposed on port 50054
kube-eleven:
	GOLANG_LOG=debug PROMETHEUS_PORT=9095 go run ./services/kube-eleven/cmd/worker

# Start Kuber service on a local environment, exposed on port 50057
kuber:
	GOLANG_LOG=debug PROMETHEUS_PORT=9096 go run ./services/kuber/cmd/worker

# Start Claudie-operator service on a local environment
# This is not necessary to have running on local environtment, to inject input manifest,
# use API directly from either /services/manager
operator:
	GOLANG_LOG=debug go run ./services/claudie-operator

# Start the database for configs, containing input manifests
mongo:
	mkdir -p ~/mongo/data
	docker run --name mongo -d --rm -p 27017:27017 -v ~/mongo/data:/data/db mongo:5

nats:
	mkdir -p ~/nats
	docker run --name nats -d --rm -p 4222:4222 -v ~/nats:/data nats -js -sd /data

# Start minio backend for state files used in terraform
minio:
# mkdir will simulate the automatic bucket creation
	mkdir -p ~/minio/data/claudie-tf-state-files
	docker run --name minio -d --rm -p 9000:9000 -p 9001:9001 --name minio -v ~/minio/data:/data quay.io/minio/minio server /data --console-address ":9001"

# Start Testing-framework, which will inject manifests from /services/testing-framework/test-sets
# -timeout 0 will disable default timeout
# Successful test will end with infrastructure being destroyed
test:
	AUTO_CLEAN_UP=TRUE GOLANG_LOG=debug go test -v ./services/testing-framework -timeout 0 -count=1 -run TestClaudie

# Run the golang linter
lint:
	golangci-lint run

# Start all data stores at once,in docker containers, to simplify the local development
datastoreStart: mongo minio nats dockernetwork

dockernetwork:
	docker network create claudie-test-network
	docker network connect claudie-test-network nats
	docker network connect claudie-test-network minio
	docker network connect claudie-test-network mongo

# Stops all data stores at once, which will also remove docker containers
datastoreStop:
	docker network disconnect claudie-test-network nats || true
	docker network disconnect claudie-test-network minio || true
	docker network disconnect claudie-test-network mongo || true
	docker network rm claudie-test-network
	docker stop mongo
	docker stop minio
	docker stop nats

# We need the value of local architecture to pass to docker as a build arg and
# Go already needs to be installed so we make use of it here.
# Use sed to set the image tag for cluster adapter, clean up at the end.
TARGETARCH = $$(go env GOHOSTARCH)
REV = $$(git rev-parse --short HEAD)
SERVICES = $$(command ls services/)
# macOS (BSD) sed requires -i '' while GNU sed uses -i
SED_INPLACE := $(shell if [ "$$(uname)" = "Darwin" ]; then echo "sed -i ''"; else echo "sed -i"; fi)
containerimgs:
	$(SED_INPLACE) "s/image: ghcr.io\/berops\/claudie\/autoscaler-adapter/&:$(REV)/" services/manager/internal/service/managementcluster/internal/autoscaler/cluster-autoscaler.goyaml
	for service in $(SERVICES) ; do \
		echo " --- building $$service --- "; \
		DOCKER_BUILDKIT=1 docker build --build-arg=TARGETARCH="$(TARGETARCH)" -t "ghcr.io/berops/claudie/$$service:$(REV)" -f ./services/$$service/Dockerfile . ; \
	done
	$(SED_INPLACE) "s/adapter:.*$$/adapter/" services/manager/internal/service/managementcluster/internal/autoscaler/cluster-autoscaler.goyaml

KIND_CLUSTER ?= kind
KIND_NAMESPACE ?= claudie
kind-load-images:
	for service in $(SERVICES) ; do \
		echo " --- loading $$service to kind cluster --- "; \
		kind load docker-image --name $(KIND_CLUSTER) ghcr.io/berops/claudie/$$service:$(REV); \
	done

kind-deploy: kind-load-images
	@echo " --- updating deployments in $(KIND_NAMESPACE) namespace --- "
	@for svc in ansibler claudie-operator kube-eleven kuber manager terraformer; do \
		echo " --- updating $$svc deployment --- "; \
		kubectl set image deployment/$$svc $$svc=ghcr.io/berops/claudie/$$svc:$(REV) -n $(KIND_NAMESPACE); \
	done
	@echo " --- waiting for rollouts to complete --- "
	@kubectl rollout status deployment -n $(KIND_NAMESPACE)

# Generate CustomResourceDefinition objects.
crd:
	go tool controller-gen rbac:roleName=manager-role crd paths="./..." output:crd:artifacts:config=manifests/claudie/crd

crd-apply:
	go tool controller-gen crd paths="./..." output:crd:artifacts:config=manifests/claudie/crd && kubectl apply -k ./manifests/claudie/crd/
