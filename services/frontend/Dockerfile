FROM docker.io/library/golang:1.20.4 AS build

ARG TARGETARCH

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
WORKDIR /go/services/frontend

#Compile the golang code, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

#Use empty base image
FROM scratch
#Add repository label
LABEL org.opencontainers.image.source "https://github.com/berops/claudie"
#Add image name as a label
LABEL org.opencontainers.image.base.name "scratch"
#Add description to the image
LABEL org.opencontainers.image.description "Image for Frontend from Claudie"

#Copy the binaries to empty base image
COPY --from=build  /go/services/frontend/frontend /bin/services/frontend/frontend

WORKDIR /bin
#Run server
ENTRYPOINT [ "./services/frontend/frontend" ]