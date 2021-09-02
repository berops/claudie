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
WORKDIR ./services/builder

#Compile the golang code to /out, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build -o /out

#Install kubectl
RUN wget https://storage.googleapis.com/kubernetes-release/release/v1.18.0/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl
RUN mv ./kubectl /usr/local/bin/kubectl
RUN kubectl version --client

#Run server
ENTRYPOINT [ "/out" ]

#Note: When deleting nodes, builder is using kubectl drain/delete from shell