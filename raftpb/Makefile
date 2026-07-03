
all: proto

proto:
	rm -f *.swagger.json
	protoc *.proto -I . -I third_party -I ~/go/include/google/protobuf --go_out=. --go-grpc_out=. --grpc-gateway_out=logtostderr=true,allow_delete_body=true:. --openapiv2_out=logtostderr=true,allow_delete_body=true:.

update:
	go get -u ./...


