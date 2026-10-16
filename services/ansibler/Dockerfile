FROM docker.io/library/golang:1.27.2 AS go_builder

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
WORKDIR /go/services/ansibler/cmd/worker

#Compile the golang code, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

FROM alpine:3.24

#Add repository label
LABEL org.opencontainers.image.source="https://github.com/berops/claudie"
#Add image name as a label
LABEL org.opencontainers.image.base.name="alpine:3.24"
#Add description to the image
LABEL org.opencontainers.image.description="Image for Ansibler from Claudie"

# Ansible installation: https://github.com/cytopia/docker-ansible/blob/master/Dockerfiles/Dockerfile-base
# Ansible is installed straight into the system Python of the final stage, so there is no
# /usr/lib/python3.X path to keep in sync with the Python version shipped by the alpine base image.
RUN set -eux \
	&& apk add --no-cache \
	# tini init system for cleaning up zombie processes left by spawned ansible playbooks \
	# https://github.com/berops/claudie/issues/1317
	tini \
	bash \
	# Issue: #85 libgcc required for ansible-vault
	libgcc \
	py3-pip \
	python3 \
	openssh-client \
	# Issue: #76 yaml required for 'libyaml = True' (faster startup time)
	yaml \
	&& ln -sf /usr/bin/python3 /usr/bin/python \
	# ansible 13.x ships ansible-core 2.20, which supports the Python 3.14 of alpine 3.24.
	&& pip3 install --no-cache-dir --break-system-packages ansible==13.8.0 \
	# Pre-compile Python for better performance
	&& python3 -m compileall -q -j 0 /usr/lib/python3.*

#Copy the binaries to empty base image
COPY --from=go_builder /go/services/ansibler/cmd/worker/worker /bin/services/ansibler/worker
COPY --from=go_builder /go/services/ansibler/ansible-playbooks /bin/services/ansibler/ansible-playbooks

#Run worker
WORKDIR /bin
ENTRYPOINT ["/sbin/tini", "--"]
CMD [ "./services/ansibler/worker" ]
#NOTE: We cannot use scratch image for our dockerfile since we are using shell commands to execute commands inside the code
