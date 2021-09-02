FROM golang:1.16-alpine AS build

#Unset the GOPATH
ENV GOPATH=

#First, copy go.mod and go.sum to prevent uneccesary download of modules
COPY go.mod .
COPY go.sum .

#Check if any modules need downloading
RUN go mod download

#install dependecies for tests
RUN apk add build-base

#Copy all files apart from the ones in .dockerignore
COPY . .

#Change the directory
WORKDIR ./services/testing-framework

#Run server
ENTRYPOINT [ "go", "test", "-v", "./..." ]
