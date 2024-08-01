FROM docker.io/library/golang:1.22.5 AS go_builder

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

# Ansible installation: https://github.com/cytopia/docker-ansible/blob/master/Dockerfiles/Dockerfile-base
FROM docker.io/library/alpine:3.20 AS ansible_builder

RUN set -eux \
	&& apk add --update --no-cache \
	# build tools
	coreutils \
	g++ \
	gcc \
	make \
	musl-dev \
	openssl-dev \
	python3-dev \
	# misc tools
	bc \
	libffi-dev \
	libxml2-dev \
	libxslt-dev \
	py3-pip \
	python3 \
	# Fix: ansible --version: libyaml = True
	# https://www.jeffgeerling.com/blog/2021/ansible-might-be-running-slow-if-libyaml-not-available
	&& apk add --update --no-cache \
	py3-yaml \
	&& python3 -c 'import _yaml'

# Pip required tools
RUN set -eux \
	&& pip3 install --no-cache-dir --no-compile \
	wheel
RUN set -eux \
	&& pip3 install --no-cache-dir --no-compile \
	Jinja2 \
	MarkupSafe \
	PyNaCl \
	bcrypt \
	cffi \
	cryptography \
	pycparser

RUN set -eux \
	# ansible 10.1.0 includes ansible-core 2.17 and ansible-community 9.6.0
	&& pip3 install --no-cache-dir --no-binary pyyaml ansible==10.1.0 \
	&& find /usr/lib/ -name '__pycache__' -print0 | xargs -0 -n1 rm -rf \
	&& find /usr/lib/ -name '*.pyc' -print0 | xargs -0 -n1 rm -rf

# Python packages (copied to final image)
RUN set -eux \
	&& pip3 install --no-cache-dir --no-compile \
	junit-xml \
	lxml \
	paramiko \
	&& find /usr/lib/ -name '__pycache__' -print0 | xargs -0 -n1 rm -rf \
	&& find /usr/lib/ -name '*.pyc' -print0 | xargs -0 -n1 rm -rf

# Clean-up some site-packages to safe space
RUN set -eux \
	&& pip3 uninstall --yes \
	setuptools \
	wheel \
	&& find /usr/lib/ -name '__pycache__' -print0 | xargs -0 -n1 rm -rf \
	&& find /usr/lib/ -name '*.pyc' -print0 | xargs -0 -n1 rm -rf

FROM alpine:3.20
#Add repository label
LABEL org.opencontainers.image.source "https://github.com/berops/claudie"
#Add image name as a label
LABEL org.opencontainers.image.base.name "alpine:3.16"
#Add description to the image
LABEL org.opencontainers.image.description "Image for Ansibler from Claudie"

RUN set -eux \
	&& apk add --no-cache \
	# Issue: #85 libgcc required for ansible-vault
	libgcc \
	py3-pip \
	python3 \
	openssh-client \
	# Issue: #76 yaml required for 'libyaml = True' (faster startup time)
	yaml \
	&& find /usr/lib/ -name '__pycache__' -print0 | xargs -0 -n1 rm -rf \
	&& find /usr/lib/ -name '*.pyc' -print0 | xargs -0 -n1 rm -rf \
	&& ln -s /usr/bin/python3 /usr/bin/python

COPY --from=ansible_builder /usr/lib/python3.10/site-packages/ /usr/lib/python3.10/site-packages/
COPY --from=ansible_builder /usr/bin/ansible* /usr/bin/

# Pre-compile Python for better performance
RUN set -eux \
	&& python3 -m compileall -j 0 /usr/lib/python3.10

#Copy the binaries to empty base image
COPY --from=go_builder /go/services/ansibler/server/server /bin/services/ansibler/server/server
COPY --from=go_builder /go/services/ansibler/server/ansible-playbooks /bin/services/ansibler/server/ansible-playbooks

RUN apk update && apk add -q bash

#Run server
WORKDIR /bin
ENTRYPOINT [ "./services/ansibler/server/server" ]
#NOTE: We cannot use scratch image for our dockerfile since we are using shell commands to execute commands inside the code
