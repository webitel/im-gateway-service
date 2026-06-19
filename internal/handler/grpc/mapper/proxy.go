package mapper

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
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

func ConvertByProtoReflect(src, dst protoreflect.Message) {
	src.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		dstFd := dst.Descriptor().Fields().ByNumber(fd.Number())
		if dstFd == nil {
			return true
		}

		if fd.IsList() != dstFd.IsList() || fd.IsMap() != dstFd.IsMap() {
			return true
		}

		if fd.Kind() == protoreflect.MessageKind && dstFd.Kind() == protoreflect.MessageKind {
			if fd.IsList() {
				copyListMessages(v.List(), dst.Mutable(dstFd).List(), dstFd)
			} else if fd.IsMap() {
				copyMapMessages(v.Map(), dst.Mutable(dstFd).Map(), dstFd)
			} else {
				newDstVal := dst.NewField(dstFd)
				newDstMsg := newDstVal.Message()

				ConvertByProtoReflect(v.Message(), newDstMsg)

				dst.Set(dstFd, newDstVal)
			}
			return true
		}

		if fd.Kind() == dstFd.Kind() {
			dst.Set(dstFd, v)
		}

		return true
	})
}

func copyListMessages(srcList, dstList protoreflect.List, _ protoreflect.FieldDescriptor) {
	for i := 0; i < srcList.Len(); i++ {
		srcItem := srcList.Get(i).Message()

		newDstItem := dstList.NewElement().Message()
		ConvertByProtoReflect(srcItem, newDstItem)

		dstList.Append(protoreflect.ValueOfMessage(newDstItem))
	}
}

func copyMapMessages(srcMap, dstMap protoreflect.Map, _ protoreflect.FieldDescriptor) {
	srcMap.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		srcValueMsg := v.Message()

		newDstValue := dstMap.NewValue().Message()
		ConvertByProtoReflect(srcValueMsg, newDstValue)

		dstMap.Set(k, protoreflect.ValueOfMessage(newDstValue))
		return true
	})
}
