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
WORKDIR /go/services/builder

#Compile the golang code to /out, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

FROM alpine

#Install kubectl
RUN wget https://storage.googleapis.com/kubernetes-release/release/v1.18.0/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl
RUN mv ./kubectl /usr/local/bin/kubectl
RUN kubectl version --client

#Copy the binaries to empty base image
COPY --from=build /go/services/builder /bin/services/builder

#Run server
WORKDIR /bin
ENTRYPOINT [ "./services/builder/builder" ]

#Note: When deleting nodes, builder is using kubectl drain/delete from shell