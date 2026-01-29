package buf

//go:generate buf generate ../../protos/im --template buf.gen.contact.yaml --path ../../protos/im/service/contact/v1

//go:generate buf generate ../../protos/im --template buf.gen.contact.yaml --path ../../protos/im/domain/contact/v1
