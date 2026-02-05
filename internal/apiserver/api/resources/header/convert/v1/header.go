package headerv1

import (
	"time"

	"github.com/loicsikidi/holistik/internal/apiserver/api/resources/header"
	headerv1 "github.com/loicsikidi/holistik/proto/gen/proto/go/holistik/common/header/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// FromMetadataProto converts v1 metadata into an internal go struct.
func FromMetadataProto(msg *headerv1.Metadata) header.Metadata {
	if msg == nil {
		return header.Metadata{}
	}

	var expires time.Time
	if msg.GetExpires() != nil {
		expires = msg.GetExpires().AsTime()
	}

	return header.Metadata{
		Name:        msg.GetName(),
		Description: msg.GetDescription(),
		Labels:      msg.GetLabels(),
		Expires:     expires,
		Revision:    msg.GetRevision(),
	}
}

// ToMetadataProto converts an internal go struct into a v1 metadata protobuf message.
func ToMetadataProto(metadata header.Metadata) *headerv1.Metadata {
	var expires *timestamppb.Timestamp
	if !metadata.Expires.IsZero() {
		expires = timestamppb.New(metadata.Expires)
	}

	return &headerv1.Metadata{
		Name:        metadata.Name,
		Description: metadata.Description,
		Labels:      metadata.Labels,
		Expires:     expires,
		Revision:    metadata.Revision,
	}
}
