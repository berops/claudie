FROM docker.io/library/golang:1.20.4 AS build

ARG TARGETARCH

#Install kubectl
RUN KC_VERSION=v1.24.0 && \
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
WORKDIR /go/services/testing-framework
RUN CGO_ENABLED=0 go test -c -run TestClaudie

FROM docker.io/library/alpine:3.17.3
#Add repository label
LABEL org.opencontainers.image.source "https://github.com/berops/claudie"
#Add image name as a label
LABEL org.opencontainers.image.base.name "docker.io/library/alpine"
#Add description to the image
LABEL org.opencontainers.image.description "Image for Testing-framework from Claudie"

COPY --from=build /go/kubectl /usr/local/bin/kubectl
COPY --from=build /go/services/testing-framework/testing-framework.test /go/services/testing-framework/testing-framework.test
COPY --from=build /go/services/testing-framework/templates /go/services/testing-framework/templates

RUN chmod +x /usr/local/bin/kubectl && \
    apk add -q bash

#Run server
WORKDIR /go/services/testing-framework
ENTRYPOINT [ "./testing-framework.test", "-test.run", "TestClaudie", "-test.timeout=10800s", "-test.v", "./..." ]
