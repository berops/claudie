FROM golang:alpine AS build

#unset the GOPATH
ENV GOPATH=

#First, copy go.mod and go.sum to prevent uneccesary dowload of modules
COPY go.mod .
COPY go.sum .

#Check if any modules need downloading
RUN go mod download

#Copy all files apart from the ones in .dockerignore
COPY . .

#change the directory
WORKDIR ./services/context-box/server

#compile the golang code
RUN go build

#run server
CMD ["./server"]
