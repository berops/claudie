FROM docker.io/library/golang:1.20.3 AS build

ARG TARGETARCH

#Unset the GOPATH
ENV GOPATH=

#First, copy go.mod and go.sum to prevent uneccesary download of modules
COPY go.mod .
COPY go.sum .

#Check if any modules need downloading
RUN go mod download

COPY . .

#Change the directory
WORKDIR /go/services/ansibler/server

#Compile the golang code, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

#Use alpine based ansible image
FROM docker.io/cytopia/ansible:2.13-0.50
#Add repository label
LABEL org.opencontainers.image.source "https://github.com/berops/claudie"
#Add image name as a label
LABEL org.opencontainers.image.base.name "docker.io/cytopia/ansible:2.13-0.50"
#Add description to the image
LABEL org.opencontainers.image.description "Image for Ansibler from Claudie"

#Copy the binaries to empty base image
COPY --from=build /go/services/ansibler/server/server /bin/services/ansibler/server/server
COPY --from=build /go/services/ansibler/templates /bin/services/ansibler/templates
COPY --from=build /go/services/ansibler/server/ansible-playbooks /bin/services/ansibler/server/ansible-playbooks

RUN apk update && apk add -q bash

#Run server
WORKDIR /bin
ENTRYPOINT [ "./services/ansibler/server/server" ]
#NOTE: We cannot use scratch image for our dockerfile since we are using shell commands to execute commands inside the code
