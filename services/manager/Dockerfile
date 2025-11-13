FROM docker.io/library/golang:1.25.4 AS build

ARG TARGETARCH

#Install kubectl
RUN KC_VERSION=v1.27.0 && \
    wget -q https://storage.googleapis.com/kubernetes-release/release/${KC_VERSION}/bin/linux/$TARGETARCH/kubectl

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
WORKDIR /go/services/manager/cmd/api-server

#Compile the golang code, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

#Use alpine image to copy certs needed for cloud libraries
FROM alpine:latest as certs
RUN apk --update add ca-certificates

FROM docker.io/library/alpine:3.22
#Add repository label
LABEL org.opencontainers.image.source="https://github.com/berops/claudie"
#Add base image name as a label
LABEL org.opencontainers.image.base.name="docker.io/library/alpine"
#Add description to the image
LABEL org.opencontainers.image.description="Image for Manager from Claudie"

#Copy the binaries to empty base image
COPY --from=build /go/kubectl /usr/local/bin/kubectl
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build  /go/services/manager/cmd/api-server/api-server /bin/services/manager/api-server

WORKDIR /bin

RUN chmod +x /usr/local/bin/kubectl && apk add -q bash

#Run server
ENTRYPOINT [ "./services/manager/api-server" ]
