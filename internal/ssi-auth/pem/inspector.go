// inspector implements the wallet.PemInspector port for validating key material.
// It inspects PEM-encoded cryptographic keys without exposing JOSE details to domain.
package keys

import (
	"errors"
	"fmt"

	"github.com/caparicio-esd/alexandria/internal/ssi-auth/jose"
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/wallet"
)

// PemInspector implements wallet.KeyMaterial on top of the jose ACL.
type PemInspector struct{}

// NewInspector builds the inspector. There is nothing to configure; the
// constructor is here so the composition root reads like every other wiring.
func NewInspector() PemInspector { return PemInspector{} }

// Inspect derives a domain descriptor from PEM material, translating the jose
// package's verdicts into the wallet's vocabulary. It implements
// wallet.PemInspector.
func (PemInspector) Inspect(material string) (wallet.PemDescriptor, error) {
	key, err := jose.ParsePEM([]byte(material))
	if err != nil {
		return wallet.PemDescriptor{}, err
	}

	// The thumbprint is computed over the public half in both cases, so a key
	// and its own public counterpart answer with the same value. That is what
	// makes it usable as an identity for the pair rather than for the encoding.
	thumbprint, err := jose.Thumbprint(key)
	if err != nil {
		return wallet.PemDescriptor{}, errors.New("holds a key that cannot be thumbprinted")
	}

	private, err := jose.IsPrivate(key)
	if err != nil {
		return wallet.PemDescriptor{}, fmt.Errorf("names a key type this build cannot use: %s", key.KeyType())
	}

	descriptor := wallet.PemDescriptor{
		Thumbprint: thumbprint,
		Kty:        key.KeyType().String(),
		Crv:        nil,
		Private:    private,
	}

	// RSA keys carry no "crv", and that is not a failure — hence the boolean
	// rather than an error. The curve is checked against the registry even
	// though jwx has already parsed it, because the registry this build was
	// compiled with is the narrower of the two: secp256k1 is only registered
	// under the jwx_es256k build tag.
	if crv, ok := jose.CurveOf(key); ok {
		if _, err := jose.ParseCrv(crv.String()); err != nil {
			return wallet.PemDescriptor{}, fmt.Errorf("is on a curve this build was not compiled with: %s", crv)
		}

		name := crv.String()
		descriptor.Crv = &name
	}

	return descriptor, nil
}
