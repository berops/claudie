FROM docker.io/library/golang:1.25.3 AS build

ARG TARGETARCH

#Install kubectl
RUN KC_VERSION=v1.33.0 && \
    wget -q https://dl.k8s.io/release/${KC_VERSION}/bin/linux/$TARGETARCH/kubectl

#Unset the GOPATH
ENV GOPATH=

#First, copy go.mod and go.sum to prevent uneccesary download of modules
COPY go.mod .
COPY go.sum .

#Check if any modules need downloading
RUN go mod download

COPY . .

#Change the directory
WORKDIR /go/services/kuber/cmd/worker

#Compile the golang code, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

FROM docker.io/library/alpine:3.22
#Add repository label
LABEL org.opencontainers.image.source="https://github.com/berops/claudie"
#Add image name as a label
LABEL org.opencontainers.image.base.name="docker.io/library/alpine"
#Add description to the image
LABEL org.opencontainers.image.description="Image for Kuber from Claudie"

#Copy the binaries to empty base image
COPY --from=build /go/kubectl /usr/local/bin/kubectl
COPY --from=build /go/services/kuber/cmd/worker/worker /bin/services/kuber/worker
COPY --from=build /go/services/kuber/manifests /bin/services/kuber/manifests
RUN chmod +x /usr/local/bin/kubectl \
    && apk update \
    && apk add -q bash \
    && rm -rf /var/cache/apk/*

#Run worker
WORKDIR /bin
ENTRYPOINT [ "./services/kuber/worker" ]

#NOTE: We cannot use scratch image for our dockerfile since we are using shell commands to execute commands inside the code
