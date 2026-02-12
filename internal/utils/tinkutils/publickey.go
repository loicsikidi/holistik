package tinkutils

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"math/big"
	"slices"

	"github.com/loicsikidi/sentinel"
	tinkecdsa "github.com/tink-crypto/tink-go/v2/signature/ecdsa"
	tinked25519 "github.com/tink-crypto/tink-go/v2/signature/ed25519"
	tinkrsassapkcs1 "github.com/tink-crypto/tink-go/v2/signature/rsassapkcs1"
	tinkrsassapss "github.com/tink-crypto/tink-go/v2/signature/rsassapss"
)

// PublicKey extracts a standard Go [crypto.PublicKey] from a Tink public key.
//
// Supported Tink key types:
//   - [ecdsa.PublicKey]
//   - [rsassapkcs1.PublicKey]
//   - [rsassapss.PublicKey]
//   - [ed25519.PublicKey]
func PublicKey(pub any) (crypto.PublicKey, error) {
	switch key := pub.(type) {
	case *tinkecdsa.PublicKey:
		return convertECDSAPublicKey(key)
	case *tinkrsassapkcs1.PublicKey:
		return convertRSAPublicKey(key.Modulus(), key.Parameters())
	case *tinkrsassapss.PublicKey:
		return convertRSAPublicKey(key.Modulus(), key.Parameters())
	case *tinked25519.PublicKey:
		return convertEd25519PublicKey(key)
	default:
		return nil, sentinel.BadParameter("unsupported key type: %T", pub)
	}
}

// PublicKeyRSA extracts an [rsa.PublicKey] from a Tink public key.
func PublicKeyRSA(pub any) (*rsa.PublicKey, error) {
	cryptoPub, err := PublicKey(pub)
	if err != nil {
		return nil, sentinel.Wrap(err)
	}

	rsaPub, ok := cryptoPub.(*rsa.PublicKey)
	if !ok {
		return nil, sentinel.BadParameter("expected RSA key, got %T", cryptoPub)
	}

	return rsaPub, nil
}

// PublicKeyECDSA extracts an [ecdsa.PublicKey] from a Tink public key.
func PublicKeyECDSA(pub any) (*ecdsa.PublicKey, error) {
	cryptoPub, err := PublicKey(pub)
	if err != nil {
		return nil, sentinel.Wrap(err)
	}

	ecdsaPub, ok := cryptoPub.(*ecdsa.PublicKey)
	if !ok {
		return nil, sentinel.BadParameter("expected ECDSA key, got %T", cryptoPub)
	}

	return ecdsaPub, nil
}

// PublicKeyEd25519 extracts an [ed25519.PublicKey] from a Tink public key.
func PublicKeyEd25519(pub any) (ed25519.PublicKey, error) {
	cryptoPub, err := PublicKey(pub)
	if err != nil {
		return nil, sentinel.Wrap(err)
	}

	ed25519Pub, ok := cryptoPub.(ed25519.PublicKey)
	if !ok {
		return nil, sentinel.BadParameter("expected Ed25519 key, got %T", cryptoPub)
	}

	return ed25519Pub, nil
}

// convertECDSAPublicKey converts a Tink ECDSA public key to a standard Go [ecdsa.PublicKey].
func convertECDSAPublicKey(tinkKey *tinkecdsa.PublicKey) (*ecdsa.PublicKey, error) {
	params, ok := tinkKey.Parameters().(*tinkecdsa.Parameters)
	if !ok {
		return nil, sentinel.BadParameter("expected *ecdsa.Parameters, got %T", tinkKey.Parameters())
	}

	curve, err := curveFromTinkECDSACurveType(params.CurveType())
	if err != nil {
		return nil, sentinel.Wrap(err)
	}

	publicPoint := tinkKey.PublicPoint()
	if len(publicPoint) == 0 || publicPoint[0] != 0x04 {
		return nil, sentinel.BadParameter("invalid public point format")
	}

	// Skip the 0x04 prefix and split X, Y coordinates
	xy := publicPoint[1:]
	if len(xy)%2 != 0 {
		return nil, sentinel.BadParameter("invalid point length")
	}

	halfLen := len(xy) / 2
	x := new(big.Int).SetBytes(xy[:halfLen])
	y := new(big.Int).SetBytes(xy[halfLen:])

	return &ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}, nil
}

// curveFromTinkECDSACurveType converts a Tink ECDSA curve type to a standard Go [elliptic.Curve].
func curveFromTinkECDSACurveType(curveType tinkecdsa.CurveType) (elliptic.Curve, error) {
	switch curveType {
	case tinkecdsa.NistP256:
		return elliptic.P256(), nil
	case tinkecdsa.NistP384:
		return elliptic.P384(), nil
	case tinkecdsa.NistP521:
		return elliptic.P521(), nil
	default:
		return nil, sentinel.BadParameter("unsupported curve type: %v", curveType)
	}
}

// convertRSAPublicKey converts a Tink RSA public key to a standard Go [rsa.PublicKey].
// Supports both RSA-SSA-PKCS1 and RSA-SSA-PSS keys.
func convertRSAPublicKey(modulusBytes []byte, params any) (*rsa.PublicKey, error) {
	modulus := new(big.Int).SetBytes(modulusBytes)

	var exponent int
	switch p := params.(type) {
	case *tinkrsassapkcs1.Parameters:
		exponent = p.PublicExponent()
	case *tinkrsassapss.Parameters:
		exponent = p.PublicExponent()
	default:
		return nil, sentinel.BadParameter("expected *rsassapkcs1.Parameters or *rsassapss.Parameters, got %T", params)
	}

	return &rsa.PublicKey{
		N: modulus,
		E: exponent,
	}, nil
}

// convertEd25519PublicKey converts a Tink Ed25519 public key to a standard Go [ed25519.PublicKey].
func convertEd25519PublicKey(tinkKey *tinked25519.PublicKey) (ed25519.PublicKey, error) {
	keyBytes := tinkKey.KeyBytes()
	if len(keyBytes) != ed25519.PublicKeySize {
		return nil, sentinel.BadParameter("invalid Ed25519 key size: got %d, want %d",
			len(keyBytes), ed25519.PublicKeySize)
	}

	return slices.Clone(keyBytes), nil
}
