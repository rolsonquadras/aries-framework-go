/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package tinkcrypto provides the default implementation of the
// common pkg/common/api/crypto.Crypto interface and the SPI pkg/framework/aries.crypto interface
//
// It uses github.com/tink/go crypto primitives
package tinkcrypto

import (
	"errors"
	"fmt"

	"github.com/google/tink/go/aead"
	aeadsubtle "github.com/google/tink/go/aead/subtle"
	"github.com/google/tink/go/core/primitiveset"
	"github.com/google/tink/go/keyset"
	"github.com/google/tink/go/mac"
	"github.com/google/tink/go/signature"
	"golang.org/x/crypto/chacha20poly1305"

	cryptoapi "github.com/hyperledger/aries-framework-go/pkg/crypto"
)

const (
	// ECDHESA256KWAlg is the ECDH-ES with AES-GCM 256 key wrapping algorithm.
	ECDHESA256KWAlg = "ECDH-ES+A256KW"
	// ECDH1PUA256KWAlg is the ECDH-1PU with AES-GCM 256 key wrapping algorithm.
	ECDH1PUA256KWAlg = "ECDH-1PU+A256KW"
	// ECDHESXC20PKWAlg is the ECDH-ES with XChacha20Poly1305 key wrapping algorithm.
	ECDHESXC20PKWAlg = "ECDH-ES+XC20PKW"
	// ECDH1PUXC20PKWAlg is the ECDH-1PU with XChacha20Poly1305 key wrapping algorithm.
	ECDH1PUXC20PKWAlg = "ECDH-1PU+XC20PKW"

	nistPECDHKWPrivateKeyTypeURL  = "type.hyperledger.org/hyperledger.aries.crypto.tink.NistPEcdhKwPrivateKey"
	x25519ECDHKWPrivateKeyTypeURL = "type.hyperledger.org/hyperledger.aries.crypto.tink.X25519EcdhKwPrivateKey"
)

var errBadKeyHandleFormat = errors.New("bad key handle format")

// Package tinkcrypto includes the default implementation of pkg/crypto. It uses Tink for executing crypto primitives
// and will be built as a framework option. It represents the main crypto service in the framework. `kh interface{}`
// arguments in this implementation represent Tink's `*keyset.Handle`, using this type provides easy integration with
// Tink and the default KMS service.

// Crypto is the default Crypto SPI implementation using Tink.
type Crypto struct {
	ecKW  keyWrapper
	okpKW keyWrapper
}

// New creates a new Crypto instance.
func New() (*Crypto, error) {
	return &Crypto{ecKW: &ecKWSupport{}, okpKW: &okpKWSupport{}}, nil
}

// Encrypt will encrypt msg using the implementation's corresponding encryption key and primitive in kh of a public key.
func (t *Crypto) Encrypt(msg, aad []byte, kh interface{}) ([]byte, []byte, error) {
	keyHandle, ok := kh.(*keyset.Handle)
	if !ok {
		return nil, nil, errBadKeyHandleFormat
	}

	ps, err := keyHandle.Primitives()
	if err != nil {
		return nil, nil, fmt.Errorf("get primitives: %w", err)
	}

	a, err := aead.New(keyHandle)
	if err != nil {
		return nil, nil, fmt.Errorf("create new aead: %w", err)
	}

	ct, err := a.Encrypt(msg, aad)
	if err != nil {
		return nil, nil, fmt.Errorf("encrypt msg: %w", err)
	}

	// Tink appends a key prefix + nonce to ciphertext, let's remove them to get the raw ciphertext
	ivSize := nonceSize(ps)
	prefixLength := len(ps.Primary.Prefix)
	cipherText := ct[prefixLength+ivSize:]
	nonce := ct[prefixLength : prefixLength+ivSize]

	return cipherText, nonce, nil
}

func nonceSize(ps *primitiveset.PrimitiveSet) int {
	var ivSize int
	// AESGCM and XChacha20Poly1305 nonce sizes supported only for now
	switch ps.Primary.Primitive.(type) {
	case *aeadsubtle.XChaCha20Poly1305:
		ivSize = chacha20poly1305.NonceSizeX
	case *aeadsubtle.AESGCM:
		ivSize = aeadsubtle.AESGCMIVSize
	default:
		ivSize = aeadsubtle.AESGCMIVSize
	}

	return ivSize
}

// Decrypt will decrypt cipher using the implementation's corresponding encryption key referenced by kh of
// a private key.
func (t *Crypto) Decrypt(cipher, aad, nonce []byte, kh interface{}) ([]byte, error) {
	keyHandle, ok := kh.(*keyset.Handle)
	if !ok {
		return nil, errBadKeyHandleFormat
	}

	ps, err := keyHandle.Primitives()
	if err != nil {
		return nil, fmt.Errorf("get primitives: %w", err)
	}

	a, err := aead.New(keyHandle)
	if err != nil {
		return nil, fmt.Errorf("create new aead: %w", err)
	}

	// since Tink expects the key prefix + nonce as the ciphertext prefix, prepend them prior to calling its Decrypt()
	ct := make([]byte, 0, len(ps.Primary.Prefix)+len(nonce)+len(cipher))
	ct = append(ct, ps.Primary.Prefix...)
	ct = append(ct, nonce...)
	ct = append(ct, cipher...)

	pt, err := a.Decrypt(ct, aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt cipher: %w", err)
	}

	return pt, nil
}

// Sign will sign msg using the implementation's corresponding signing key referenced by kh of a private key.
func (t *Crypto) Sign(msg []byte, kh interface{}) ([]byte, error) {
	keyHandle, ok := kh.(*keyset.Handle)
	if !ok {
		return nil, errBadKeyHandleFormat
	}

	signer, err := signature.NewSigner(keyHandle)
	if err != nil {
		return nil, fmt.Errorf("create new signer: %w", err)
	}

	s, err := signer.Sign(msg)
	if err != nil {
		return nil, fmt.Errorf("sign msg: %w", err)
	}

	return s, nil
}

// Verify will verify sig signature of msg using the implementation's corresponding signing key referenced by kh of
// a public key.
func (t *Crypto) Verify(sig, msg []byte, kh interface{}) error {
	keyHandle, ok := kh.(*keyset.Handle)
	if !ok {
		return errBadKeyHandleFormat
	}

	verifier, err := signature.NewVerifier(keyHandle)
	if err != nil {
		return fmt.Errorf("create new verifier: %w", err)
	}

	err = verifier.Verify(sig, msg)
	if err != nil {
		err = fmt.Errorf("verify msg: %w", err)
	}

	return err
}

// ComputeMAC computes message authentication code (MAC) for code data
// using a matching MAC primitive in kh key handle.
func (t *Crypto) ComputeMAC(data []byte, kh interface{}) ([]byte, error) {
	keyHandle, ok := kh.(*keyset.Handle)
	if !ok {
		return nil, errBadKeyHandleFormat
	}

	macPrimitive, err := mac.New(keyHandle)
	if err != nil {
		return nil, err
	}

	return macPrimitive.ComputeMAC(data)
}

// VerifyMAC determines if mac is a correct authentication code (MAC) for data
// using a matching MAC primitive in kh key handle and returns nil if so, otherwise it returns an error.
func (t *Crypto) VerifyMAC(macBytes, data []byte, kh interface{}) error {
	keyHandle, ok := kh.(*keyset.Handle)
	if !ok {
		return errBadKeyHandleFormat
	}

	macPrimitive, err := mac.New(keyHandle)
	if err != nil {
		return err
	}

	return macPrimitive.VerifyMAC(macBytes, data)
}

// WrapKey will do ECDH (ES or 1PU) key wrapping of cek using apu, apv and recipient public key 'recPubKey'.
// This function is used with the following parameters:
//  - Key Wrapping: `ECDH-ES` (no options) or `ECDH-1PU` (using crypto.WithSender() option in wrapKeyOpts) over either
//    `A256KW` alg (AES256-GCM, default with no options) as per https://tools.ietf.org/html/rfc7518#appendix-A.2 or
//    `XC20PKW` alg (XChacha20Poly1305, using crypto.WithXC20PKW() option in wrapKeyOpts).
//  - KDF (based on recPubKey.Curve): `Concat KDF` as per https://tools.ietf.org/html/rfc7518#section-4.6 (for recPubKey
//    with NIST P curves) or `Curve25519`+`Concat KDF` as per https://tools.ietf.org/html/rfc7748#section-6.1 (for
//    recPubKey with X25519 curve).
// returns the resulting key wrapping info as *composite.RecipientWrappedKey or error in case of wrapping failure.
func (t *Crypto) WrapKey(cek, apu, apv []byte, recPubKey *cryptoapi.PublicKey,
	wrapKeyOpts ...cryptoapi.WrapKeyOpts) (*cryptoapi.RecipientWrappedKey, error) {
	if recPubKey == nil {
		return nil, errors.New("wrapKey: recipient public key is required")
	}

	pOpts := cryptoapi.NewOpt()

	for _, opt := range wrapKeyOpts {
		opt(pOpts)
	}

	wk, err := t.deriveKEKAndWrap(cek, apu, apv, pOpts.SenderKey(), recPubKey, pOpts.UseXC20PKW())
	if err != nil {
		return nil, fmt.Errorf("wrapKey: %w", err)
	}

	return wk, nil
}

// UnwrapKey unwraps a key in recWK using ECDH (ES or 1PU) with recipient private key kh.
// This function is used with the following parameters:
//  - Key Unwrapping: `ECDH-ES` (no options) or `ECDH-1PU` (using crypto.WithSender() option in wrapKeyOpts) over either
//    `A256KW` alg (AES256-GCM, when recWK.alg is either ECDH-ES+A256KW or ECDH-1PU+A256KW) as per
//     https://tools.ietf.org/html/rfc7518#appendix-A.2 or
//    `XC20PKW` alg (XChacha20Poly1305, when recWK.alg is either ECDH-ES+XC20PKW or ECDH-1PU+XC20PKW).
//  - KDF (based on recWk.EPK.KeyType): `Concat KDF` as per https://tools.ietf.org/html/rfc7518#section-4.6 (for type
//    value as EC) or `Curve25519`+`Concat KDF` as per https://tools.ietf.org/html/rfc7748#section-6.1 (for type value
//    as OKP, ie X25519 key).
// returns the resulting unwrapping key or error in case of unwrapping failure.
//
// Notes:
// 1- if the crypto.WithSender() option was used in WrapKey(), then it must be set here as well for successful key
//    unwrapping.
// 2- unwrapping a key with recWK.alg value set as either `ECDH-1PU+A256KW` or `ECDH-1PU+XC20PKW` requires the use of
//    crypto.WithSender() option (containing the sender public key) in order to execute ECDH-1PU derivation.
// 3- the ephemeral key in recWK.EPK must have the same KeyType as the recipientKH and the same Curve for NIST P
//    curved keys. Unwrapping a key with non matching types/curves will result in unwrapping failure.
// 4- recipientKH must contain the private key since unwrapping is usually done on the recipient side.
func (t *Crypto) UnwrapKey(recWK *cryptoapi.RecipientWrappedKey, recipientKH interface{},
	wrapKeyOpts ...cryptoapi.WrapKeyOpts) ([]byte, error) {
	if recWK == nil {
		return nil, fmt.Errorf("unwrapKey: RecipientWrappedKey is empty")
	}

	pOpts := cryptoapi.NewOpt()

	for _, opt := range wrapKeyOpts {
		opt(pOpts)
	}

	key, err := t.deriveKEKAndUnwrap(recWK.Alg, recWK.EncryptedCEK, recWK.APU, recWK.APV, &recWK.EPK, pOpts.SenderKey(),
		recipientKH)
	if err != nil {
		return nil, fmt.Errorf("unwrapKey: %w", err)
	}

	return key, nil
}
