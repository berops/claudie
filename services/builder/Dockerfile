FROM golang:1.16.15 AS build

#Install kubectl
RUN wget -q https://storage.googleapis.com/kubernetes-release/release/v1.18.0/bin/linux/amd64/kubectl
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
WORKDIR /go/services/builder

#Compile the golang code, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

FROM alpine

#Copy the binaries to empty base image
COPY --from=build /go/kubectl /usr/local/bin/kubectl
COPY --from=build /go/services/builder/builder /bin/services/builder/builder
RUN chmod +x /usr/local/bin/kubectl && \
    apk add bash

#Run server
WORKDIR /bin
ENTRYPOINT [ "./services/builder/builder" ]

#Note: When deleting nodes, builder is using kubectl drain/delete from shell