package mapper

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var m = protojson.MarshalOptions{EmitUnpopulated: false}
var u = protojson.UnmarshalOptions{DiscardUnknown: true}

func Convert[T proto.Message](src proto.Message, dst T) (T, error) {
    b, err := m.Marshal(src)
    if err != nil {
        return dst, err
    }
    return dst, u.Unmarshal(b, dst)
}