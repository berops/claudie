FROM golang:1.16.15 AS build

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
WORKDIR /go/services/scheduler

#Compile the golang code, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

#Use empty base image
FROM scratch

#Copy the binaries to empty base image
COPY --from=build /go/services/scheduler /bin/services/scheduler

WORKDIR /bin
#Run scheduler
ENTRYPOINT [ "./services/scheduler/scheduler" ]
