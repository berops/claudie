FROM docker.io/library/golang:1.20.4 AS build

ARG TARGETARCH

# download and unzip kube-one binary
RUN apt-get -qq update && apt-get -qq install unzip
RUN VERSION=1.4.4 && \
    wget -q https://releases.hashicorp.com/terraform/${VERSION}/terraform_${VERSION}_linux_$TARGETARCH.zip && \
    unzip -qq terraform_${VERSION}_linux_$TARGETARCH.zip -d terraform

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

FROM docker.io/library/alpine:3.18.0
#Add repository label
LABEL org.opencontainers.image.source "https://github.com/berops/claudie"
#Add image name as a label
LABEL org.opencontainers.image.base.name "docker.io/library/alpine"
#Add description to the image
LABEL org.opencontainers.image.description "Image for Terraformer from Claudie"

#Copy the binaries to empty base image
COPY --from=build /go/terraform/terraform /usr/local/bin

COPY --from=build /go/services/terraformer/server/server /bin/services/terraformer/server/server
COPY --from=build /go/services/terraformer/templates /bin/services/terraformer/templates

RUN apk add bash

#Run server
WORKDIR /bin
ENTRYPOINT [ "./services/terraformer/server/server" ]

#NOTE: We cannot use scratch image for our dockerfile since we are using shell commands to execute commands inside the code
