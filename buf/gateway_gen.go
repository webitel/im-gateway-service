package buf

//go:generate buf generate ../../protos/im --template buf.gen.gateway.yaml --path ../../protos/im/domain/auth/v1
//go:generate buf generate ../../protos/im --template buf.gen.gateway.yaml --path ../../protos/im/api/auth/v1
//go:generate buf generate ../../protos/im --template buf.gen.gateway.yaml --path ../../protos/im/api/thread/v1
