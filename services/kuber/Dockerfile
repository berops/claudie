FROM golang:1.19.4 AS build

#Install kubectl
RUN wget -q https://storage.googleapis.com/kubernetes-release/release/v1.22.0/bin/linux/amd64/kubectl

#Unset the GOPATH
ENV GOPATH=

#First, copy go.mod and go.sum to prevent uneccesary download of modules
COPY go.mod .
COPY go.sum .

#Check if any modules need downloading
RUN go mod download

#Install grpc health probe
RUN GRPC_HEALTH_PROBE_VERSION=v0.4.14 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

COPY ./internal/grpc-health-probe-Liveness ./grpc-health-probe-Liveness

#Change workdir to modified health probe directory
WORKDIR /go/grpc-health-probe-Liveness

#Compile modified health probe
RUN CGO_ENABLED=0 go build -o /bin/grpc_health_probe-Liveness
RUN chmod +x /bin/grpc_health_probe-Liveness

#Copy all files apart from the ones in .dockerignore
WORKDIR /go
COPY . .

#Change the directory
WORKDIR /go/services/kuber/server

#Compile the golang code, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

FROM alpine
#Add repository label
LABEL org.opencontainers.image.source "https://github.com/berops/claudie"
#Add image name as a label
LABEL org.opencontainers.image.base.name "docker.io/library/alpine"
#Add description to the image
LABEL org.opencontainers.image.description "Image for Kuber from Claudie"

#Copy the binaries to empty base image
COPY --from=build /go/kubectl /usr/local/bin/kubectl
COPY --from=build /go/services/kuber/server/server /bin/services/kuber/server/server
COPY --from=build /go/services/kuber/server/manifests /bin/services/kuber/server/manifests
COPY --from=build /go/services/kuber/templates /bin/services/kuber/templates
COPY --from=build /bin/grpc_health_probe /bin/grpc_health_probe
COPY --from=build /bin/grpc_health_probe-Liveness /bin/grpc_health_probe-Liveness
RUN chmod +x /usr/local/bin/kubectl && apk add -q bash

#Run server
WORKDIR /bin
ENTRYPOINT [ "./services/kuber/server/server" ]

#NOTE: We cannot use scratch image for our dockerfile since we are using shell commands to execute commands inside the code