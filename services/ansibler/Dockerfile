FROM golang:1.19.3 AS build

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

#Copy all files apart from the ones in .dockerignore
WORKDIR /go
COPY . .

#Change the directory
WORKDIR /go/services/ansibler/server

#Compile the golang code, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

#Use alpine based ansible image
FROM cytopia/ansible:2.13

#Copy the binaries to empty base image
COPY --from=build /go/services/ansibler/server/server /bin/services/ansibler/server/server
COPY --from=build /bin/grpc_health_probe /bin/grpc_health_probe
COPY --from=build /bin/grpc_health_probe-Liveness /bin/grpc_health_probe-Liveness
COPY --from=build /go/services/ansibler/templates /bin/services/ansibler/templates
COPY --from=build /go/services/ansibler/server/ansible-playbooks /bin/services/ansibler/server/ansible-playbooks

RUN apk update && apk add -q bash

#Run server
WORKDIR /bin
ENTRYPOINT [ "./services/ansibler/server/server" ]
#NOTE: We cannot use scratch image for our dockerfile since we are using shell commands to execute commands inside the code
