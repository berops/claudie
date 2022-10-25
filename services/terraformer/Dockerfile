FROM golang:1.18 AS build

# download and unzip kube-one binary
RUN apt-get -qq update && apt-get -qq install unzip
RUN VERSION=1.3.2 && \
    wget -q https://releases.hashicorp.com/terraform/${VERSION}/terraform_${VERSION}_linux_amd64.zip && \
    unzip -qq terraform_${VERSION}_linux_amd64.zip -d terraform

#Unset the GOPATH
ENV GOPATH=

#First, copy go.mod and go.sum to prevent uneccesary download of modules
COPY go.mod .
COPY go.sum .

#Check if any modules need downloading
RUN go mod download

#Install grpc health probe
RUN GRPC_HEALTH_PROBE_VERSION=v0.4.4 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

COPY ./internal/grpc-health-probe-Liveness ./grpc-health-probe-Liveness

#Change workdir to modified health probe directory
WORKDIR /go/grpc-health-probe-Liveness

#Compile modified health probe
RUN CGO_ENABLED=0 go build -o /bin/grpc_health_probe-Liveness
RUN chmod +x /bin/grpc_health_probe-Liveness

WORKDIR /go
#Copy all files apart from the ones in .dockerignore
COPY . .

#Change the directory
WORKDIR /go/services/terraformer/server

#Compile the golang code to /out, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

FROM alpine

#Copy the binaries to empty base image
COPY --from=build /go/terraform/terraform /usr/local/bin

COPY --from=build /go/services/terraformer/server/server /bin/services/terraformer/server/server
COPY --from=build /go/services/terraformer/templates /bin/services/terraformer/templates
COPY --from=build /bin/grpc_health_probe /bin/grpc_health_probe
COPY --from=build /bin/grpc_health_probe-Liveness /bin/grpc_health_probe-Liveness

RUN apk add bash

#Run server
WORKDIR /bin
ENTRYPOINT [ "./services/terraformer/server/server" ]

#NOTE: We cannot use scratch image for our dockerfile since we are using shell commands to execute commands inside the code
