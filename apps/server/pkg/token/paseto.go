package token

import (
	"fmt"
	"time"

	"aidanwoods.dev/go-paseto"
)

type Maker struct {
	symmetricKey paseto.V4SymmetricKey
	implicit     []byte
}

func NewMaker(hexKey string, implicit []byte) (*Maker, error) {
	var key paseto.V4SymmetricKey
	var err error

	hexKeyLength := 64 // 32 bytes hex encoded

	if len(hexKey) == hexKeyLength {
		key, err = paseto.V4SymmetricKeyFromHex(hexKey)
	} else {
		key = paseto.NewV4SymmetricKey()
	}

	if err != nil {
		return nil, err
	}

	return &Maker{
		symmetricKey: key,
		implicit:     implicit,
	}, nil
}

func (m *Maker) CreateToken(subject string, duration time.Duration) (string, error) {
	token := paseto.NewToken()
	token.SetSubject(subject)
	token.SetIssuedAt(time.Now())
	token.SetNotBefore(time.Now())
	token.SetExpiration(time.Now().Add(duration))
	token.SetFooter([]byte("footer"))

	encrypted := token.V4Encrypt(m.symmetricKey, m.implicit)
	return encrypted, nil
}

func (m *Maker) VerifyToken(tokenString string) (*paseto.Token, error) {
	parser := paseto.NewParser()

	token, err := parser.ParseV4Local(m.symmetricKey, tokenString, m.implicit)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	return token, nil
}
