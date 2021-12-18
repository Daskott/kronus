package auth

import (
	"fmt"

	"github.com/Daskott/kronus/server/auth/key"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

type KronusTokenClaims struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	IsAdmin   bool   `json:"is_admin"`
	jwt.StandardClaims
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func EncodeJWT(claims KronusTokenClaims, keyPair *key.KeyPair) (string, error) {
	token := jwt.NewWithClaims(jwt.GetSigningMethod("RS256"), claims)

	tokenString, err := token.SignedString(keyPair.PrivateKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func DecodeJWT(tokenString string, keyPair *key.KeyPair) (*KronusTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &KronusTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return keyPair.PublicKey, nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid jwt: %v", err)
	}

	tokenClaims, ok := token.Claims.(*KronusTokenClaims)
	if !ok {
		return nil, fmt.Errorf("unable to assert token.Claims to KronusTokenClaims")
	}

	return tokenClaims, nil
}
