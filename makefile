protoc-gen:
	rm -rf ./proto/generated

	mkdir ./proto/generated

	protoc --go_out=./proto/generated --go-grpc_out=./proto/generated --experimental_allow_proto3_optional \
		--go-grpc_opt=paths=source_relative --go_opt=paths=source_relative \
		--proto_path=./proto ./proto/*.proto

	cd ./proto/generated && \
		go mod init claudie/proto/generated && \
		go mod tidy