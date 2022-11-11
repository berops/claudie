#Ansible base image inspired by https://github.com/cytopia/docker-ansible
FROM alpine:3.16 as ansible-base

# Package required tools
RUN set -eux \
  && apk add --update --no-cache bc cargo cmake curl g++ \
  gcc git libffi-dev libxml2-dev libxslt-dev make musl-dev \
  openssl-dev py3-pip python3 python3-dev rust

# Fix: ansible --version: libyaml = True
# https://www.jeffgeerling.com/blog/2021/ansible-might-be-running-slow-if-libyaml-not-available
RUN set -eux \
  && apk add --update --no-cache \
  py3-yaml && python3 -c 'import _yaml'

# Pip required tools for all flavour's "builder" stage
RUN set -eux \ 
  && pip3 install --no-cache-dir --no-compile \
  wheel && pip3 install --no-cache-dir --no-compile \
  Jinja2 MarkupSafe PyNaCl \
  bcrypt cffi cryptography pycparser

# Install ansible
RUN set -eux \
  && pip3 install --no-cache-dir --no-binary pyyaml "ansible-core>=2.11,<2.12" \
  && ansible --version | grep ^ansible | grep -E "2.11\.[0-9]+" \
  && ansible-playbook --version | grep ^ansible | grep -E "2.11\.[0-9]+" \
  && ansible-galaxy --version | grep ^ansible | grep -E "2.11\.[0-9]+" \
  && find /usr/lib/ -name '__pycache__' -print0 | xargs -0 -n1 rm -rf \
  && find /usr/lib/ -name '*.pyc' -print0 | xargs -0 -n1 rm -rf

# Python packages (copied to final image)
RUN set -eux \
  && apk add --no-cache  \
  && pip3 install --no-cache-dir --no-compile junit-xml lxml paramiko \
  && find /usr/lib/ -name '__pycache__' -print0 | xargs -0 -n1 rm -rf \
  && find /usr/lib/ -name '*.pyc' -print0 | xargs -0 -n1 rm -rf

# Cleanup
RUN set -eux \
  && pip3 uninstall --yes setuptools wheel

FROM golang:1.19.2 AS build

#Unset the GOPATH
ENV GOPATH=

#First, copy go.mod and go.sum to prevent uneccesary download of modules
COPY go.mod .
COPY go.sum .

#Check if any modules need downloading
RUN go mod download

#Install grpc health probe
RUN GRPC_HEALTH_PROBE_VERSION=v0.4.4 && \
  wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
  chmod +x /bin/grpc_health_probe

COPY ./internal/grpc-health-probe-Liveness ./grpc-health-probe-Liveness

#Change workdir to modified health probe directory
WORKDIR /go/grpc-health-probe-Liveness

#Compile modified health probe
RUN CGO_ENABLED=0 go build -o /bin/grpc_health_probe-Liveness
RUN chmod +x /bin/grpc_health_probe-Liveness

#Copy all files apart from the ones in .dockerignore
WORKDIR /go
COPY . .

#Change the directory
WORKDIR /go/services/ansibler/server

#Compile the golang code, CGO_ENABLE=0 removes cross compile dependencies
RUN CGO_ENABLED=0 go build

#Use alpine based ansible image
FROM alpine:3.16

RUN set -eux \
  && apk add --no-cache \
  libgcc py3-pip python3 yaml \
  && ln -sf /usr/bin/python3 /usr/bin/python \
  && ln -sf ansible /usr/bin/ansible-config \
  && ln -sf ansible /usr/bin/ansible-console \
  && ln -sf ansible /usr/bin/ansible-doc \
  && ln -sf ansible /usr/bin/ansible-galaxy \
  && ln -sf ansible /usr/bin/ansible-inventory \
  && ln -sf ansible /usr/bin/ansible-playbook \
  && ln -sf ansible /usr/bin/ansible-pull \
  && ln -sf ansible /usr/bin/ansible-test \
  && ln -sf ansible /usr/bin/ansible-vault \
  && find /usr/lib/ -name '__pycache__' -print0 | xargs -0 -n1 rm -rf \
  && find /usr/lib/ -name '*.pyc' -print0 | xargs -0 -n1 rm -rf

#Copy the binaries to base image
COPY --from=build /go/services/ansibler/server/server /bin/services/ansibler/server/server
COPY --from=build /bin/grpc_health_probe /bin/grpc_health_probe
COPY --from=build /bin/grpc_health_probe-Liveness /bin/grpc_health_probe-Liveness
COPY --from=build /go/services/ansibler/templates /bin/services/ansibler/templates
COPY --from=build /go/services/ansibler/server/ansible-playbooks /bin/services/ansibler/server/ansible-playbooks

#Copy ansible
COPY --from=ansible-base /usr/lib/python3.10/site-packages/ /usr/lib/python3.10/site-packages/
COPY --from=ansible-base /usr/bin/ansible /usr/bin/ansible
COPY --from=ansible-base /usr/bin/ansible-connection /usr/bin/ansible-connection

# Pre-compile Python for better performance
RUN set -eux && python3 -m compileall /usr/lib/python3.10

RUN apk update && apk add -q bash

#Run server
WORKDIR /bin
ENTRYPOINT [ "./services/ansibler/server/server" ]
#NOTE: We cannot use scratch image for our dockerfile since we are using shell commands to execute commands inside the code
