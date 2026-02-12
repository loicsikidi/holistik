package tinkutils

import (
	"bytes"

	"github.com/loicsikidi/sentinel"
	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyset"
)

// InsecureMarshal marshals a [keyset.Handle] into a byte buffer.
//
// Important: the keyset will be in clear text, so do it fully aware of the consequences.
//
// Note: the buffer is a protobuf-encoded keyset (i.e. tinkpb.Keyset)
func InsecureMarshal(handle *keyset.Handle) ([]byte, error) {
	buf := &bytes.Buffer{}
	writer := keyset.NewBinaryWriter(buf)

	if err := insecurecleartextkeyset.Write(handle, writer); err != nil {
		return nil, sentinel.Wrap(err)
	}

	return buf.Bytes(), nil
}

type LoadConfig struct {
	// InsecureBuffer is a protobuf-encoded keyset generally produced by [InsecureMarshal]
	//
	// Required.
	InsecureBuffer []byte
}

func (c *LoadConfig) CheckAndSetDefaults() error {
	if len(c.InsecureBuffer) == 0 {
		return sentinel.BadParameter("missing parameter InsecureBuffer")
	}
	return nil
}

// LoadManager loads a [keyset.Manager] from a marshalled keyset.
//
// This function should support insecure/encrypted keyset.
func LoadManager(cfg LoadConfig) (*keyset.Manager, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}

	reader := keyset.NewBinaryReader(bytes.NewBuffer(cfg.InsecureBuffer))
	handle, err := insecurecleartextkeyset.Read(reader)
	if err != nil {
		return nil, sentinel.Wrap(err)
	}

	return keyset.NewManagerFromHandle(handle), nil
}
