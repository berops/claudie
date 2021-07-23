FROM golang:alpine AS build

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
WORKDIR ./services/context-box/server

#Compile the golang code to /out, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build -o /out

RUN apk update && apk add ca-certificates wget && update-ca-certificates

RUN apk update && apk add wget

#Install grpc health probe
RUN GRPC_HEALTH_PROBE_VERSION=v0.3.1 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

RUN ls

#Use empty base image
FROM scratch

#Copy the binaries to empty base image
COPY --from=build /out /out
COPY --from=build /bin/grpc_health_probe /bin/grpc_health_probe


#Run server
ENTRYPOINT [ "/out" ]