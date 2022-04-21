FROM golang:1.16.15 AS build

#Install kubectl
RUN wget https://storage.googleapis.com/kubernetes-release/release/v1.18.0/bin/linux/amd64/kubectl
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

RUN apt update
RUN apt install -y build-essential
RUN chmod +x /usr/local/bin/kubectl
RUN apt-get update && apt-get install jq

#Run server
ENTRYPOINT [ "go", "test", "-timeout=10800s", "-v", "./..." ]
