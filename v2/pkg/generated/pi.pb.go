// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        (unknown)
// source: pi/v2/pi.proto

package generated

import (
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GetDigitRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Zero-based index of the fractional digit of pi to return.
	Index         uint64 `protobuf:"varint,1,opt,name=index,proto3" json:"index,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetDigitRequest) Reset() {
	*x = GetDigitRequest{}
	mi := &file_pi_v2_pi_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetDigitRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetDigitRequest) ProtoMessage() {}

func (x *GetDigitRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pi_v2_pi_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetDigitRequest.ProtoReflect.Descriptor instead.
func (*GetDigitRequest) Descriptor() ([]byte, []int) {
	return file_pi_v2_pi_proto_rawDescGZIP(), []int{0}
}

func (x *GetDigitRequest) GetIndex() uint64 {
	if x != nil {
		return x.Index
	}
	return 0
}

type GetDigitMetadata struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Identity of the pi server; usually the hostname as reported by OS.
	Identity string `protobuf:"bytes,1,opt,name=identity,proto3" json:"identity,omitempty"`
	// List of string tags that were provided by the Pi Service configuration.
	Tags []string `protobuf:"bytes,2,rep,name=tags,proto3" json:"tags,omitempty"`
	// Map of key:value string pairs that were provided by the Pi Service configuration.
	Annotations   map[string]string `protobuf:"bytes,3,rep,name=annotations,proto3" json:"annotations,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetDigitMetadata) Reset() {
	*x = GetDigitMetadata{}
	mi := &file_pi_v2_pi_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetDigitMetadata) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetDigitMetadata) ProtoMessage() {}

func (x *GetDigitMetadata) ProtoReflect() protoreflect.Message {
	mi := &file_pi_v2_pi_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetDigitMetadata.ProtoReflect.Descriptor instead.
func (*GetDigitMetadata) Descriptor() ([]byte, []int) {
	return file_pi_v2_pi_proto_rawDescGZIP(), []int{1}
}

func (x *GetDigitMetadata) GetIdentity() string {
	if x != nil {
		return x.Identity
	}
	return ""
}

func (x *GetDigitMetadata) GetTags() []string {
	if x != nil {
		return x.Tags
	}
	return nil
}

func (x *GetDigitMetadata) GetAnnotations() map[string]string {
	if x != nil {
		return x.Annotations
	}
	return nil
}

type GetDigitResponse struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	// Zero-based index of the fractional digit of pi being returned.
	Index uint64 `protobuf:"varint,1,opt,name=index,proto3" json:"index,omitempty"`
	// Fractional digit of pi at request offset; this is always an unsigned integer
	// between 0 and 9 inclusive
	Digit uint32 `protobuf:"varint,2,opt,name=digit,proto3" json:"digit,omitempty"`
	// Metadata from the pi service that handled the request
	Metadata      *GetDigitMetadata `protobuf:"bytes,3,opt,name=metadata,proto3" json:"metadata,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetDigitResponse) Reset() {
	*x = GetDigitResponse{}
	mi := &file_pi_v2_pi_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetDigitResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetDigitResponse) ProtoMessage() {}

func (x *GetDigitResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pi_v2_pi_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetDigitResponse.ProtoReflect.Descriptor instead.
func (*GetDigitResponse) Descriptor() ([]byte, []int) {
	return file_pi_v2_pi_proto_rawDescGZIP(), []int{2}
}

func (x *GetDigitResponse) GetIndex() uint64 {
	if x != nil {
		return x.Index
	}
	return 0
}

func (x *GetDigitResponse) GetDigit() uint32 {
	if x != nil {
		return x.Digit
	}
	return 0
}

func (x *GetDigitResponse) GetMetadata() *GetDigitMetadata {
	if x != nil {
		return x.Metadata
	}
	return nil
}

var File_pi_v2_pi_proto protoreflect.FileDescriptor

const file_pi_v2_pi_proto_rawDesc = "" +
	"\n" +
	"\x0epi/v2/pi.proto\x12\x05pi.v2\x1a\x1cgoogle/api/annotations.proto\x1a.protoc-gen-openapiv2/options/annotations.proto\"'\n" +
	"\x0fGetDigitRequest\x12\x14\n" +
	"\x05index\x18\x01 \x01(\x04R\x05index\"\xce\x01\n" +
	"\x10GetDigitMetadata\x12\x1a\n" +
	"\bidentity\x18\x01 \x01(\tR\bidentity\x12\x12\n" +
	"\x04tags\x18\x02 \x03(\tR\x04tags\x12J\n" +
	"\vannotations\x18\x03 \x03(\v2(.pi.v2.GetDigitMetadata.AnnotationsEntryR\vannotations\x1a>\n" +
	"\x10AnnotationsEntry\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x12\x14\n" +
	"\x05value\x18\x02 \x01(\tR\x05value:\x028\x01\"s\n" +
	"\x10GetDigitResponse\x12\x14\n" +
	"\x05index\x18\x01 \x01(\x04R\x05index\x12\x14\n" +
	"\x05digit\x18\x02 \x01(\rR\x05digit\x123\n" +
	"\bmetadata\x18\x03 \x01(\v2\x17.pi.v2.GetDigitMetadataR\bmetadata2g\n" +
	"\tPiService\x12Z\n" +
	"\bGetDigit\x12\x16.pi.v2.GetDigitRequest\x1a\x17.pi.v2.GetDigitResponse\"\x1d\x82\xd3\xe4\x93\x02\x17\x12\x15/api/v2/digit/{index}B\x82\x02\x92A\xd8\x01\x12\x80\x01\n" +
	"\x02pi\"=\n" +
	"\fMatthew Emes\x12-https://github.com/memes/pi/issues/new/choose*4\n" +
	"\x03MIT\x12-https://github.com/memes/pi/blob/main/LICENSE2\x052.0.0*\x02\x01\x022\x10application/json:\x10application/jsonr+\n" +
	"\vGitHub repo\x12\x1chttps://github.com/memes/pi/Z$github.com/memes/pi/v2/pkg/generatedb\x06proto3"

var (
	file_pi_v2_pi_proto_rawDescOnce sync.Once
	file_pi_v2_pi_proto_rawDescData []byte
)

func file_pi_v2_pi_proto_rawDescGZIP() []byte {
	file_pi_v2_pi_proto_rawDescOnce.Do(func() {
		file_pi_v2_pi_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_pi_v2_pi_proto_rawDesc), len(file_pi_v2_pi_proto_rawDesc)))
	})
	return file_pi_v2_pi_proto_rawDescData
}

var file_pi_v2_pi_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_pi_v2_pi_proto_goTypes = []any{
	(*GetDigitRequest)(nil),  // 0: pi.v2.GetDigitRequest
	(*GetDigitMetadata)(nil), // 1: pi.v2.GetDigitMetadata
	(*GetDigitResponse)(nil), // 2: pi.v2.GetDigitResponse
	nil,                      // 3: pi.v2.GetDigitMetadata.AnnotationsEntry
}
var file_pi_v2_pi_proto_depIdxs = []int32{
	3, // 0: pi.v2.GetDigitMetadata.annotations:type_name -> pi.v2.GetDigitMetadata.AnnotationsEntry
	1, // 1: pi.v2.GetDigitResponse.metadata:type_name -> pi.v2.GetDigitMetadata
	0, // 2: pi.v2.PiService.GetDigit:input_type -> pi.v2.GetDigitRequest
	2, // 3: pi.v2.PiService.GetDigit:output_type -> pi.v2.GetDigitResponse
	3, // [3:4] is the sub-list for method output_type
	2, // [2:3] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_pi_v2_pi_proto_init() }
func file_pi_v2_pi_proto_init() {
	if File_pi_v2_pi_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_pi_v2_pi_proto_rawDesc), len(file_pi_v2_pi_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_pi_v2_pi_proto_goTypes,
		DependencyIndexes: file_pi_v2_pi_proto_depIdxs,
		MessageInfos:      file_pi_v2_pi_proto_msgTypes,
	}.Build()
	File_pi_v2_pi_proto = out.File
	file_pi_v2_pi_proto_goTypes = nil
	file_pi_v2_pi_proto_depIdxs = nil
}
