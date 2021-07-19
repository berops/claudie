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

#Install grpc health probes
RUN wget -q /bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.4/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

#Use empty base image
FROM scratch

#Copy the binaries to empty base image
COPY --from=build /out /out
COPY --from=build /bin/grpc_health_probe /bin/grpc_health_probe

#Run server
ENTRYPOINT [ "/out" ]