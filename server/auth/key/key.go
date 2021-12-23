package key

import (
	"crypto/rsa"
	"fmt"

	"github.com/golang-jwt/jwt"
	"github.com/lestrrat-go/jwx/jwk"
)

type JWKS struct {
	Keys []interface{} `json:"keys"`
}

type KeyPair struct {
	Kid        string
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

func NewKeyPairFromRSAPrivateKeyPem(rawKeyPem string) (*KeyPair, error) {
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(rawKeyPem))
	if err != nil {
		return nil, fmt.Errorf("unable to parse RSA private key: %v", err)
	}

	return &KeyPair{
		Kid:        "kronus-key-id",
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey}, nil
}

func (keyPair *KeyPair) JWK() (jwk.Key, error) {
	keyPairJWK, err := jwk.New(keyPair.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("JWK: %v", err)
	}
	keyPairJWK.Set(jwk.KeyIDKey, keyPair.Kid)

	return keyPairJWK, nil
}

func ExportJWKAsJWKS(jwk jwk.Key) JWKS {
	return JWKS{Keys: []interface{}{jwk}}
}

func PublicKeyFromJWK(key jwk.Key) (*rsa.PublicKey, error) {
	var publicKey *rsa.PublicKey

	err := key.Raw(publicKey)
	if err != nil {
		return nil, err
	}

	return publicKey, nil
}
