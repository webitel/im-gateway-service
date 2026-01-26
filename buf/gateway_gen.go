package buf

//go:generate buf generate ../../protos/im --template buf.gen.gateway.yaml --path ../../protos/im/gateway/v1/auth

//go:generate buf generate ../../protos/im --template buf.gen.gateway.yaml --path ../../protos/im/gateway/v1/contact

//go:generate buf generate ../../protos/im --template buf.gen.gateway.yaml --path ../../protos/im/gateway/v1/shared

//go:generate buf generate ../../protos/im --template buf.gen.gateway.yaml --path ../../protos/im/gateway/v1/thread
