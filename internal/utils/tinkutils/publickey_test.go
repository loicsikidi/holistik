package tinkutils_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"strings"
	"testing"

	"github.com/loicsikidi/holistik/internal/utils/tinkutils"
	"github.com/tink-crypto/tink-go/v2/keyset"
	tinkpb "github.com/tink-crypto/tink-go/v2/proto/tink_go_proto"
	"github.com/tink-crypto/tink-go/v2/signature"
	tinkecdsa "github.com/tink-crypto/tink-go/v2/signature/ecdsa"
	tinkrsassapkcs1 "github.com/tink-crypto/tink-go/v2/signature/rsassapkcs1"
	tinkrsassapss "github.com/tink-crypto/tink-go/v2/signature/rsassapss"
)

// extractTinkPublicKey extracts the Tink public key from a keyset handle
func extractTinkPublicKey(t *testing.T, handle *keyset.Handle) any {
	t.Helper()

	entry, err := handle.Entry(0)
	if err != nil {
		t.Fatalf("failed to get entry: %v", err)
	}

	return entry.Key()
}

func TestPublicKey_ECDSA(t *testing.T) {
	testCases := []struct {
		name     string
		template *tinkpb.KeyTemplate
		curve    elliptic.Curve
	}{
		{"P256", signature.ECDSAP256KeyWithoutPrefixTemplate(), elliptic.P256()},
		{"P384", signature.ECDSAP384SHA384KeyWithoutPrefixTemplate(), elliptic.P384()},
		{"P521", signature.ECDSAP521KeyWithoutPrefixTemplate(), elliptic.P521()},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := keyset.NewManager()
			keyID, err := manager.Add(tc.template)
			if err != nil {
				t.Fatalf("Add() failed: %v", err)
			}
			manager.SetPrimary(keyID)

			handle, err := manager.Handle()
			if err != nil {
				t.Fatalf("Handle() failed: %v", err)
			}

			pubHandle, err := handle.Public()
			if err != nil {
				t.Fatalf("Public() failed: %v", err)
			}

			tinkPubKey := extractTinkPublicKey(t, pubHandle)

			_, ok := tinkPubKey.(*tinkecdsa.PublicKey)
			if !ok {
				t.Fatalf("expected *tinkecdsa.PublicKey, got %T", tinkPubKey)
			}

			stdPubKey, err := tinkutils.PublicKey(tinkPubKey)
			if err != nil {
				t.Fatalf("PublicKey() failed: %v", err)
			}

			ecdsaPubKey, ok := stdPubKey.(*ecdsa.PublicKey)
			if !ok {
				t.Fatalf("expected *ecdsa.PublicKey, got %T", stdPubKey)
			}

			if ecdsaPubKey.Curve != tc.curve {
				t.Errorf("expected curve %v, got %v", tc.curve, ecdsaPubKey.Curve)
			}
		})
	}
}

func TestPublicKey_RSA(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testCases := []struct {
		name            string
		template        *tinkpb.KeyTemplate
		modulusSizeBits int
		expectedType    any
	}{
		{"RSA_PKCS1_3072_SHA256", signature.RSA_SSA_PKCS1_3072_SHA256_F4_RAW_Key_Template(), 3072, (*tinkrsassapkcs1.PublicKey)(nil)},
		{"RSA_PKCS1_4096_SHA512", signature.RSA_SSA_PKCS1_4096_SHA512_F4_RAW_Key_Template(), 4096, (*tinkrsassapkcs1.PublicKey)(nil)},
		{"RSA_PSS_3072_SHA256", signature.RSA_SSA_PSS_3072_SHA256_32_F4_Raw_Key_Template(), 3072, (*tinkrsassapss.PublicKey)(nil)},
		{"RSA_PSS_4096_SHA512", signature.RSA_SSA_PSS_4096_SHA512_64_F4_Raw_Key_Template(), 4096, (*tinkrsassapss.PublicKey)(nil)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := keyset.NewManager()
			keyID, err := manager.Add(tc.template)
			if err != nil {
				t.Fatalf("Add() failed: %v", err)
			}
			manager.SetPrimary(keyID)

			handle, err := manager.Handle()
			if err != nil {
				t.Fatalf("Handle() failed: %v", err)
			}

			pubHandle, err := handle.Public()
			if err != nil {
				t.Fatalf("Public() failed: %v", err)
			}

			tinkPubKey := extractTinkPublicKey(t, pubHandle)

			switch tc.expectedType.(type) {
			case *tinkrsassapkcs1.PublicKey:
				if _, ok := tinkPubKey.(*tinkrsassapkcs1.PublicKey); !ok {
					t.Fatalf("expected *tinkrsassapkcs1.PublicKey, got %T", tinkPubKey)
				}
			case *tinkrsassapss.PublicKey:
				if _, ok := tinkPubKey.(*tinkrsassapss.PublicKey); !ok {
					t.Fatalf("expected *tinkrsassapss.PublicKey, got %T", tinkPubKey)
				}
			}

			stdPubKey, err := tinkutils.PublicKey(tinkPubKey)
			if err != nil {
				t.Fatalf("PublicKey() failed: %v", err)
			}

			rsaPubKey, ok := stdPubKey.(*rsa.PublicKey)
			if !ok {
				t.Fatalf("expected *rsa.PublicKey, got %T", stdPubKey)
			}

			actualBits := rsaPubKey.N.BitLen()
			if actualBits < tc.modulusSizeBits-8 || actualBits > tc.modulusSizeBits {
				t.Errorf("expected modulus size around %d bits, got %d bits", tc.modulusSizeBits, actualBits)
			}
		})
	}
}

func TestPublicKey_Errors(t *testing.T) {
	testCases := []struct {
		name    string
		key     any
		wantErr string
	}{
		{"nil key", nil, "unsupported key type"},
		{"unsupported type", 42, "unsupported key type"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tinkutils.PublicKey(tc.key)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

// verifyFunc is a function that verifies a signature using the converted standard Go public key
type verifyFunc func(t *testing.T, stdPubKey crypto.PublicKey, testData, sig []byte)

func TestPublicKey_RoundTrip(t *testing.T) {
	testCases := []struct {
		name      string
		template  *tinkpb.KeyTemplate
		verify    verifyFunc
		skipShort bool
	}{
		{
			name:     "ECDSA_P256_SHA256",
			template: signature.ECDSAP256KeyWithoutPrefixTemplate(),
			verify: func(t *testing.T, stdPubKey crypto.PublicKey, testData, sig []byte) {
				t.Helper()
				ecdsaPubKey := stdPubKey.(*ecdsa.PublicKey)
				hash := sha256.Sum256(testData)
				if !ecdsa.VerifyASN1(ecdsaPubKey, hash[:], sig) {
					t.Error("signature verification failed with standard Go ECDSA")
				}
			},
		},
		{
			name:      "RSA_PKCS1_3072_SHA256",
			template:  signature.RSA_SSA_PKCS1_3072_SHA256_F4_RAW_Key_Template(),
			skipShort: true,
			verify: func(t *testing.T, stdPubKey crypto.PublicKey, testData, sig []byte) {
				t.Helper()
				rsaPubKey := stdPubKey.(*rsa.PublicKey)
				hash := sha256.Sum256(testData)
				if err := rsa.VerifyPKCS1v15(rsaPubKey, crypto.SHA256, hash[:], sig); err != nil {
					t.Errorf("signature verification failed with standard Go RSA: %v", err)
				}
			},
		},
		{
			name:      "RSA_PKCS1_4096_SHA512",
			template:  signature.RSA_SSA_PKCS1_4096_SHA512_F4_RAW_Key_Template(),
			skipShort: true,
			verify: func(t *testing.T, stdPubKey crypto.PublicKey, testData, sig []byte) {
				t.Helper()
				rsaPubKey := stdPubKey.(*rsa.PublicKey)
				hash := sha512.Sum512(testData)
				if err := rsa.VerifyPKCS1v15(rsaPubKey, crypto.SHA512, hash[:], sig); err != nil {
					t.Errorf("signature verification failed with standard Go RSA: %v", err)
				}
			},
		},
		{
			name:      "RSA_PSS_3072_SHA256",
			template:  signature.RSA_SSA_PSS_3072_SHA256_32_F4_Raw_Key_Template(),
			skipShort: true,
			verify: func(t *testing.T, stdPubKey crypto.PublicKey, testData, sig []byte) {
				t.Helper()
				rsaPubKey := stdPubKey.(*rsa.PublicKey)
				hash := sha256.Sum256(testData)
				opts := &rsa.PSSOptions{
					SaltLength: 32,
					Hash:       crypto.SHA256,
				}
				if err := rsa.VerifyPSS(rsaPubKey, crypto.SHA256, hash[:], sig, opts); err != nil {
					t.Errorf("signature verification failed with standard Go RSA-PSS: %v", err)
				}
			},
		},
		{
			name:      "RSA_PSS_4096_SHA512",
			template:  signature.RSA_SSA_PSS_4096_SHA512_64_F4_Raw_Key_Template(),
			skipShort: true,
			verify: func(t *testing.T, stdPubKey crypto.PublicKey, testData, sig []byte) {
				t.Helper()
				rsaPubKey := stdPubKey.(*rsa.PublicKey)
				hash := sha512.Sum512(testData)
				opts := &rsa.PSSOptions{
					SaltLength: 64,
					Hash:       crypto.SHA512,
				}
				if err := rsa.VerifyPSS(rsaPubKey, crypto.SHA512, hash[:], sig, opts); err != nil {
					t.Errorf("signature verification failed with standard Go RSA-PSS: %v", err)
				}
			},
		},
		{
			name:     "Ed25519",
			template: signature.ED25519KeyWithoutPrefixTemplate(),
			verify: func(t *testing.T, stdPubKey crypto.PublicKey, testData, sig []byte) {
				t.Helper()
				ed25519PubKey := stdPubKey.(ed25519.PublicKey)
				if !ed25519.Verify(ed25519PubKey, testData, sig) {
					t.Error("signature verification failed with standard Go Ed25519")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipShort && testing.Short() {
				t.Skip("Skipping integration test in short mode")
			}

			manager := keyset.NewManager()
			keyID, err := manager.Add(tc.template)
			if err != nil {
				t.Fatalf("Add() failed: %v", err)
			}
			manager.SetPrimary(keyID)

			handle, err := manager.Handle()
			if err != nil {
				t.Fatalf("Handle() failed: %v", err)
			}

			signer, err := signature.NewSigner(handle)
			if err != nil {
				t.Fatalf("NewSigner() failed: %v", err)
			}

			testData := []byte("test message for signing")
			sig, err := signer.Sign(testData)
			if err != nil {
				t.Fatalf("Sign() failed: %v", err)
			}

			pubHandle, err := handle.Public()
			if err != nil {
				t.Fatalf("Public() failed: %v", err)
			}

			tinkPubKey := extractTinkPublicKey(t, pubHandle)
			stdPubKey, err := tinkutils.PublicKey(tinkPubKey)
			if err != nil {
				t.Fatalf("PublicKey() failed: %v", err)
			}

			tc.verify(t, stdPubKey, testData, sig)
		})
	}
}

func TestPublicKeyRSA(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testCases := []struct {
		name            string
		template        *tinkpb.KeyTemplate
		modulusSizeBits int
	}{
		{"RSA_PKCS1_3072_SHA256", signature.RSA_SSA_PKCS1_3072_SHA256_F4_RAW_Key_Template(), 3072},
		{"RSA_PSS_4096_SHA512", signature.RSA_SSA_PSS_4096_SHA512_64_F4_Raw_Key_Template(), 4096},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := keyset.NewManager()
			keyID, err := manager.Add(tc.template)
			if err != nil {
				t.Fatalf("Add() failed: %v", err)
			}
			manager.SetPrimary(keyID)

			handle, err := manager.Handle()
			if err != nil {
				t.Fatalf("Handle() failed: %v", err)
			}

			pubHandle, err := handle.Public()
			if err != nil {
				t.Fatalf("Public() failed: %v", err)
			}

			tinkPubKey := extractTinkPublicKey(t, pubHandle)

			rsaPubKey, err := tinkutils.PublicKeyRSA(tinkPubKey)
			if err != nil {
				t.Fatalf("PublicKeyRSA() failed: %v", err)
			}

			actualBits := rsaPubKey.N.BitLen()
			if actualBits < tc.modulusSizeBits-8 || actualBits > tc.modulusSizeBits {
				t.Errorf("expected modulus size around %d bits, got %d bits", tc.modulusSizeBits, actualBits)
			}
		})
	}
}

func TestPublicKeyRSA_Errors(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testCases := []struct {
		name     string
		template *tinkpb.KeyTemplate
		wantErr  string
	}{
		{"ECDSA key", signature.ECDSAP256KeyWithoutPrefixTemplate(), "expected RSA key"},
		{"Ed25519 key", signature.ED25519KeyWithoutPrefixTemplate(), "expected RSA key"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := keyset.NewManager()
			keyID, err := manager.Add(tc.template)
			if err != nil {
				t.Fatalf("Add() failed: %v", err)
			}
			manager.SetPrimary(keyID)

			handle, err := manager.Handle()
			if err != nil {
				t.Fatalf("Handle() failed: %v", err)
			}

			pubHandle, err := handle.Public()
			if err != nil {
				t.Fatalf("Public() failed: %v", err)
			}

			tinkPubKey := extractTinkPublicKey(t, pubHandle)

			_, err = tinkutils.PublicKeyRSA(tinkPubKey)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}

	t.Run("nil key", func(t *testing.T) {
		_, err := tinkutils.PublicKeyRSA(nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "key is nil") {
			t.Errorf("expected error containing %q, got %q", "key is nil", err.Error())
		}
	})
}

func TestPublicKeyECDSA(t *testing.T) {
	testCases := []struct {
		name     string
		template *tinkpb.KeyTemplate
		curve    elliptic.Curve
	}{
		{"P256", signature.ECDSAP256KeyWithoutPrefixTemplate(), elliptic.P256()},
		{"P384", signature.ECDSAP384SHA384KeyWithoutPrefixTemplate(), elliptic.P384()},
		{"P521", signature.ECDSAP521KeyWithoutPrefixTemplate(), elliptic.P521()},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := keyset.NewManager()
			keyID, err := manager.Add(tc.template)
			if err != nil {
				t.Fatalf("Add() failed: %v", err)
			}
			manager.SetPrimary(keyID)

			handle, err := manager.Handle()
			if err != nil {
				t.Fatalf("Handle() failed: %v", err)
			}

			pubHandle, err := handle.Public()
			if err != nil {
				t.Fatalf("Public() failed: %v", err)
			}

			tinkPubKey := extractTinkPublicKey(t, pubHandle)

			ecdsaPubKey, err := tinkutils.PublicKeyECDSA(tinkPubKey)
			if err != nil {
				t.Fatalf("PublicKeyECDSA() failed: %v", err)
			}

			if ecdsaPubKey.Curve != tc.curve {
				t.Errorf("expected curve %v, got %v", tc.curve, ecdsaPubKey.Curve)
			}
		})
	}
}

func TestPublicKeyECDSA_Errors(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testCases := []struct {
		name     string
		template *tinkpb.KeyTemplate
		wantErr  string
	}{
		{"RSA key", signature.RSA_SSA_PKCS1_3072_SHA256_F4_RAW_Key_Template(), "expected ECDSA key"},
		{"Ed25519 key", signature.ED25519KeyWithoutPrefixTemplate(), "expected ECDSA key"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := keyset.NewManager()
			keyID, err := manager.Add(tc.template)
			if err != nil {
				t.Fatalf("Add() failed: %v", err)
			}
			manager.SetPrimary(keyID)

			handle, err := manager.Handle()
			if err != nil {
				t.Fatalf("Handle() failed: %v", err)
			}

			pubHandle, err := handle.Public()
			if err != nil {
				t.Fatalf("Public() failed: %v", err)
			}

			tinkPubKey := extractTinkPublicKey(t, pubHandle)

			_, err = tinkutils.PublicKeyECDSA(tinkPubKey)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}

	t.Run("nil key", func(t *testing.T) {
		_, err := tinkutils.PublicKeyECDSA(nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "key is nil") {
			t.Errorf("expected error containing %q, got %q", "key is nil", err.Error())
		}
	})
}

func TestPublicKeyEd25519(t *testing.T) {
	t.Run("Ed25519", func(t *testing.T) {
		manager := keyset.NewManager()
		keyID, err := manager.Add(signature.ED25519KeyWithoutPrefixTemplate())
		if err != nil {
			t.Fatalf("Add() failed: %v", err)
		}
		manager.SetPrimary(keyID)

		handle, err := manager.Handle()
		if err != nil {
			t.Fatalf("Handle() failed: %v", err)
		}

		pubHandle, err := handle.Public()
		if err != nil {
			t.Fatalf("Public() failed: %v", err)
		}

		tinkPubKey := extractTinkPublicKey(t, pubHandle)

		ed25519PubKey, err := tinkutils.PublicKeyEd25519(tinkPubKey)
		if err != nil {
			t.Fatalf("PublicKeyEd25519() failed: %v", err)
		}

		if len(ed25519PubKey) != ed25519.PublicKeySize {
			t.Errorf("expected key size %d, got %d", ed25519.PublicKeySize, len(ed25519PubKey))
		}
	})
}

func TestPublicKeyEd25519_Errors(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testCases := []struct {
		name     string
		template *tinkpb.KeyTemplate
		wantErr  string
	}{
		{"RSA key", signature.RSA_SSA_PKCS1_3072_SHA256_F4_RAW_Key_Template(), "expected Ed25519 key"},
		{"ECDSA key", signature.ECDSAP256KeyWithoutPrefixTemplate(), "expected Ed25519 key"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := keyset.NewManager()
			keyID, err := manager.Add(tc.template)
			if err != nil {
				t.Fatalf("Add() failed: %v", err)
			}
			manager.SetPrimary(keyID)

			handle, err := manager.Handle()
			if err != nil {
				t.Fatalf("Handle() failed: %v", err)
			}

			pubHandle, err := handle.Public()
			if err != nil {
				t.Fatalf("Public() failed: %v", err)
			}

			tinkPubKey := extractTinkPublicKey(t, pubHandle)

			_, err = tinkutils.PublicKeyEd25519(tinkPubKey)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}

	t.Run("nil key", func(t *testing.T) {
		_, err := tinkutils.PublicKeyEd25519(nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "key is nil") {
			t.Errorf("expected error containing %q, got %q", "key is nil", err.Error())
		}
	})
}
