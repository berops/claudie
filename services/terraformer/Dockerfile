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
WORKDIR /go/services/terraformer/server

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

#Install terraform
WORKDIR /go
RUN apt install unzip
RUN wget https://releases.hashicorp.com/terraform/1.0.0/terraform_1.0.0_linux_amd64.zip
RUN unzip terraform_1.0.0_linux_amd64.zip -d /usr/local/bin

#Run server
WORKDIR /go/services/terraformer/server
ENTRYPOINT [ "./server" ]

#NOTE: We cannot use scratch image for our dockerfile since we are using shell commands to execute commands inside the code
 