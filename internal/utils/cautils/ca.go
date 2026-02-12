package ca_utils

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"time"

	"github.com/loicsikidi/sentinel"
)

const (
	device      = "device"
	attestation = "attestation"
)

var SubjectsByType = map[string]*pkix.Name{
	device:      {CommonName: "Device Root CA", Organization: []string{"Holistik"}},
	attestation: {CommonName: "Attestation Root CA", Organization: []string{"Holistik"}},
}

// generateSerialNumber generates a cryptographically secure random serial number
// suitable for X.509 certificates.
func generateSerialNumber() (*big.Int, error) {
	// RFC 5280 recommends 20 bytes (160 bits) for serial numbers
	// See section 4.1.2.2 of RFC 5280
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 160)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, sentinel.Wrap(err)
	}
	return serialNumber, nil
}

var rootTemplate = &x509.Certificate{
	KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	IsCA:                  true,
	BasicConstraintsValid: true,
}

type RootCertificateRequest struct {
	// Subject contains the distinguished name information for the certificate.
	//
	// Required.
	Subject *pkix.Name

	// Duration specifies the validity period for the certificate.
	//
	// Optional. Defaults to 20 years.
	Duration time.Duration

	// Signer is the cryptographic signer used to sign the certificate.
	//
	// Required.
	Signer crypto.Signer
}

func (r *RootCertificateRequest) CheckAndSetDefaults() error {
	if r.Subject == nil {
		return sentinel.BadParameter("missing parameter Subject")
	}
	if r.Signer == nil {
		return sentinel.BadParameter("missing parameter Signer")
	}
	if r.Duration == 0 {
		r.Duration = 20 * 365 * 24 * time.Hour // Default to 20 years
	}
	return nil
}

func CreateRootCertificate(request RootCertificateRequest) (*x509.Certificate, error) {
	if err := request.CheckAndSetDefaults(); err != nil {
		return nil, err
	}

	serialNumber, err := generateSerialNumber()
	if err != nil {
		return nil, sentinel.Wrap(err)
	}

	// Clone rootTemplate
	template := *rootTemplate
	template.Subject = *request.Subject

	now := time.Now()
	template.NotBefore = now.Add(-1 * time.Minute)
	template.NotAfter = now.Add(request.Duration)
	template.SerialNumber = serialNumber

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, request.Signer.Public(), request.Signer)
	if err != nil {
		return nil, sentinel.Wrap(err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, sentinel.Wrap(err)
	}

	return cert, nil
}
