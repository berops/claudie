FROM golang:1.16.15 AS build

# download and unzip kube-one binary
RUN apt-get update && apt-get install unzip
RUN wget https://github.com/kubermatic/kubeone/releases/download/v1.3.3/kubeone_1.3.3_linux_amd64.zip
RUN unzip kubeone_1.3.3_linux_amd64.zip -d kubeone_1.3.3_linux_amd64
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

COPY ./grpc-health-probe-Liveness ./grpc-health-probe-Liveness

#Change workdir to modified health probe directory
WORKDIR /go/grpc-health-probe-Liveness

#Compile modified health probe
RUN CGO_ENABLED=0 go build -o /bin/grpc_health_probe-Liveness
RUN chmod +x /bin/grpc_health_probe-Liveness

#Copy all files apart from the ones in .dockerignore
WORKDIR /go
COPY . .

#Change the directory
WORKDIR /go/services/kube-eleven/server

#Compile the golang code, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

FROM alpine

COPY --from=build /go/kubeone_1.3.3_linux_amd64/kubeone /usr/local/bin
COPY --from=build /go/services/kube-eleven /bin/services/kube-eleven
COPY --from=build /bin/grpc_health_probe /bin/grpc_health_probe
COPY --from=build /bin/grpc_health_probe-Liveness /bin/grpc_health_probe-Liveness

#Run server
WORKDIR /bin
ENTRYPOINT [ "./services/kube-eleven/server/server" ]

#NOTE: We cannot use scratch image for our dockerfile since we are using shell commands to execute commands inside the code