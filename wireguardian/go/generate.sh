#!/bin/bash

protoc wireguardianpb/wireguardian.proto --go_out=plugins=grpc:.