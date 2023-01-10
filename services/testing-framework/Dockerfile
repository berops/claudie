FROM docker.io/library/golang:1.19.4 AS build

#Install kubectl
RUN wget -q https://storage.googleapis.com/kubernetes-release/release/v1.18.0/bin/linux/amd64/kubectl
RUN mv kubectl /usr/local/bin/

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

FROM docker.io/library/alpine

COPY --from=build /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=build /go/services/testing-framework/testing-framework.test /go/services/testing-framework/testing-framework.test
COPY --from=build /go/services/testing-framework/templates /go/services/testing-framework/templates

RUN chmod +x /usr/local/bin/kubectl && \
    apk add -q bash

#Run server
WORKDIR /go/services/testing-framework
ENTRYPOINT [ "./testing-framework.test", "-test.run", "TestClaudie", "-test.timeout=10800s", "-test.v", "./..." ]
