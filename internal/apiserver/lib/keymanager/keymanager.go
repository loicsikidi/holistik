package keymanager

import (
	"crypto"
	"io"

	"github.com/loicsikidi/holistik/internal/utils"
	"github.com/loicsikidi/holistik/internal/utils/tinkutils"
	typesv1 "github.com/loicsikidi/holistik/proto/gen/proto/go/holistik/api/common/types/v1"
	"github.com/loicsikidi/sentinel"
	"github.com/tink-crypto/tink-go/v2/keyset"
	"github.com/tink-crypto/tink-go/v2/signature"
)

type Config struct {
	// Backend is the backend to use for the keyset.
	//
	// Optional. Defaults to [typesv1.CAKeySet_KEYSET_BACKEND_RAW].
	Backend typesv1.CAKeySet_KeySetBackend
	// Type is the type of the keyset.
	//
	// Optional. Defaults to [typesv1.CAKeySet_KEYSET_TYPE_INSECURE].
	Type typesv1.CAKeySet_KeySetType
}

func (c *Config) CheckAndSetDefaults() error {
	if c.Backend == typesv1.CAKeySet_KEYSET_BACKEND_UNSPECIFIED {
		c.Backend = typesv1.CAKeySet_KEYSET_BACKEND_RAW
	}
	if c.Type == typesv1.CAKeySet_KEYSET_TYPE_UNSPECIFIED {
		c.Type = typesv1.CAKeySet_KEYSET_TYPE_INSECURE
	}
	return nil
}

type KeyManager interface {
	// Init initializes a key manager.
	// A keyset will be created from scratch.
	//
	// Note: use this function when a new keyset is needed.
	Init() error

	// GetSigner returns a [crypto.MessageSigner] for signing messages.
	//
	// We use this interface because [tink.Signer] only supports signing messages, not digests.
	//
	// Luckily for us, crypto package from stdlib supports this interface for signing certs and CRL.
	//
	// Note: use this function to obtain a signer for the primary key.
	GetSigner() (crypto.MessageSigner, error)
}

type keyManager struct {
	cfg     *Config
	manager *keyset.Manager
}

func NewKeyManager(optionalCfg ...Config) (KeyManager, error) {
	cfg := utils.OptionalArg(optionalCfg)
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}
	return &keyManager{cfg: &cfg}, nil
}

func (km *keyManager) Init() error {
	if km.manager != nil {
		return sentinel.AlreadyExists("key manager already initialized")
	}

	ksm := keyset.NewManager()
	keyID, err := ksm.Add(signature.ECDSAP256KeyWithoutPrefixTemplate())
	if err != nil {
		return sentinel.Wrap(err)
	}

	if err := ksm.SetPrimary(keyID); err != nil {
		return sentinel.Wrap(err)
	}

	km.manager = ksm
	return nil
}

func (km *keyManager) GetSigner() (crypto.MessageSigner, error) {
	handle, err := km.manager.Handle()
	if err != nil {
		return nil, sentinel.Wrap(err)
	}
	pk := &primaryKey{handle: handle}
	if err := pk.CheckAndSetDefaults(); err != nil {
		return nil, sentinel.Wrap(err)
	}
	return pk, nil
}

// primaryKey implements [crypto.MessageSigner] for the primary key in a keyset.
type primaryKey struct {
	handle *keyset.Handle
	public crypto.PublicKey
}

func (pk *primaryKey) CheckAndSetDefaults() error {
	if pk.handle == nil {
		return sentinel.BadParameter("missing parameter handle")
	}
	if pk.public == nil {
		primary, err := pk.handle.Primary()
		if err != nil {
			return sentinel.Wrap(err)
		}
		pub, err := tinkutils.PublicKey(primary.Key())
		if err != nil {
			return sentinel.Wrap(err)
		}
		pk.public = pub
	}
	return nil
}

var _ crypto.MessageSigner = (*primaryKey)(nil)

// Public implements the [crypto.Signer] interface.
func (pk *primaryKey) Public() crypto.PublicKey {
	return pk.public
}

// Sign implements the [crypto.Signer] interface.
func (pk *primaryKey) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	return nil, sentinel.NotImplemented("Sign is not implemented, use SignMessage instead")
}

// SignMessage implements the [crypto.MessageSigner] interface.
func (pk *primaryKey) SignMessage(_ io.Reader, msg []byte, _ crypto.SignerOpts) ([]byte, error) {
	signer, err := signature.NewSigner(pk.handle)
	if err != nil {
		return nil, sentinel.Wrap(err)
	}
	return signer.Sign(msg)
}
