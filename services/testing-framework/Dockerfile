FROM golang:1.16 AS build

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
#Run server
ENTRYPOINT [ "go", "test", "-timeout 3600s","-v", "./..." ]
