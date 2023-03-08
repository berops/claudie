FROM docker.io/library/golang:1.20.0 AS build

#Unset the GOPATH
ENV GOPATH=

#First, copy go.mod and go.sum to prevent uneccesary download of modules
COPY go.mod .
COPY go.sum .

#Check if any modules need downloading
RUN go mod download


WORKDIR /go
#Copy all files apart from the ones in .dockerignore
COPY . .
#Change the directory
WORKDIR /go/services/autoscaler-adapter

#Compile the golang code, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build -o claudie-autoscaler-adapter

#Use empty base image
FROM docker.io/library/alpine:3.17.2
#Add repository label
LABEL org.opencontainers.image.source "https://github.com/berops/claudie"
#Add base image name as a label
LABEL org.opencontainers.image.base.name "docker.io/library/alpine:3.17.2"
#Add description to the image
LABEL org.opencontainers.image.description "Image for external gRPC provider for Cluster Autoscaler by Claudie"

#Copy the binaries to empty base image
COPY --from=build  /go/services/autoscaler-adapter/claudie-autoscaler-adapter /bin/claudie-autoscaler-adapter

WORKDIR /bin

ENTRYPOINT [ "./claudie-autoscaler-adapter"]