FROM golang:1.16 AS build

#Unset the GOPATH
ENV GOPATH=

#First, copy go.mod and go.sum to prevent uneccesary download of modules
COPY go.mod .
COPY go.sum .

#Check if any modules need downloading
RUN go mod download

#Copy all files apart from the ones in .dockerignore
COPY . .

#Change the directory
WORKDIR /go/services/kube-eleven/server

#Compile the golang code to /out, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

#Install grpc health probe
RUN GRPC_HEALTH_PROBE_VERSION=v0.4.4 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

#Change workdir to modified health probe directory
WORKDIR /go/grpc-health-probe-Liveness

#Compile modified health probe
RUN CGO_ENABLED=0 go build -o /bin/grpc_health_probe-Liveness
RUN chmod +x /bin/grpc_health_probe-Liveness

# Install kubeone
WORKDIR /go
RUN apt update
RUN apt install -y unzip
RUN wget https://github.com/kubermatic/kubeone/releases/download/v1.2.3/kubeone_1.2.3_linux_amd64.zip
RUN unzip kubeone_1.2.3_linux_amd64.zip -d kubeone_1.2.3_linux_amd64
RUN mv kubeone_1.2.3_linux_amd64/kubeone /usr/local/bin

WORKDIR /go/services/kube-eleven/server
#Run server
ENTRYPOINT [ "./server" ]

#NOTE: We cannot use scratch image for our dockerfile since we are using shell commands to execute commands inside the code