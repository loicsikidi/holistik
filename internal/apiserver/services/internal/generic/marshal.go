package generic

import (
	"time"

	"github.com/loicsikidi/holistik/internal/apiserver/api/resources"
	"github.com/loicsikidi/holistik/internal/utils"
	"github.com/loicsikidi/sentinel"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ProtoResource abstracts a resource defined as a protobuf message.
type ProtoResource interface {
	proto.Message
	resources.Resource
}

// PtrToProtoResource is a ProtoResource with the constraint to be a pointer to T.
//
// See section "Pointer Receivers" from "Generic interfaces" for more context.
// Source: https://go.dev/blog/generic-interfaces#pointer-receivers
type PtrToProtoResource[T any] interface {
	ProtoResource
	*T
}

// MarshalConfig specifies marshaling options
type MarshalConfig struct {
	// Revision of the resource to assign.
	Revision string

	// Expires is an optional expiry time
	Expires time.Time

	// If DiscardUnknown is set, unknown fields and enum name values are ignored.
	DiscardUnknown bool
}

// MarshalProtoResource marshals a ProtoResource to JSON using [protojson.Marshal]
func MarshalProtoResource[T ProtoResource](resource T, _ ...MarshalConfig) ([]byte, error) {
	data, err := protojson.Marshal(resource)
	if err != nil {
		return nil, sentinel.Wrap(err)
	}
	return data, nil
}

// UnmarshalProtoResource unmarshals a ProtoResource from JSON using [protojson.Unmarshal].
func UnmarshalProtoResource[T any, PT PtrToProtoResource[T]](data []byte, optionalCfg ...MarshalConfig) (PT, error) {
	if len(data) == 0 {
		return nil, sentinel.BadParameter("nothing to unmarshal")
	}
	cfg := utils.OptionalArg(optionalCfg)

	var resource PT = new(T)
	err := protojson.UnmarshalOptions{DiscardUnknown: !cfg.DiscardUnknown}.Unmarshal(data, resource)
	if err != nil {
		return nil, sentinel.Wrap(err)
	}
	if cfg.Revision != "" {
		resource.GetMetadata().Revision = cfg.Revision
	}
	if !cfg.Expires.IsZero() {
		resource.GetMetadata().Expires = timestamppb.New(cfg.Expires)
	}
	return resource, nil
}

// WrapToUnmarshalProtoResource creates an [UnmarshalFunc] for a ProtoResource type.
//
// Note: you can use it in [ServiceConfig]
func WrapToUnmarshalProtoResource[T any, PT PtrToProtoResource[T]]() func([]byte, ...MarshalConfig) (PT, error) {
	return func(data []byte, cfg ...MarshalConfig) (PT, error) {
		return UnmarshalProtoResource[T, PT](data, cfg...)
	}
}

// WrapToUnmarshalProtoResource creates an [MarshalFunc] for a ProtoResource type.
//
// Note: you can use it in [ServiceConfig]
func WrapToMarshalProtoResource[T ProtoResource]() func(T, ...MarshalConfig) ([]byte, error) {
	return func(r T, cfg ...MarshalConfig) ([]byte, error) {
		return MarshalProtoResource(r, cfg...)
	}
}
