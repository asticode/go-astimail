package asticrypt_test

import (
	"testing"

	"github.com/asticode/go-asticrypt"
	"github.com/stretchr/testify/assert"
)

func TestEncryptedMessage(t *testing.T) {
	// Init
	var pk1, pk2 = &asticrypt.PrivateKey{}, &asticrypt.PrivateKey{}
	pk1.SetPassphrase("test")
	err := pk1.UnmarshalText([]byte(prv1))
	assert.NoError(t, err)
	err = pk2.UnmarshalText([]byte(prv2))
	assert.NoError(t, err)

	// Assert
	m, err := asticrypt.NewEncryptedMessage("test", pk2, pk1.Public())
	assert.NoError(t, err)
	var b string
	err = m.Decrypt(&b, pk1, pk2.Public())
	assert.NoError(t, err)
	assert.Equal(t, "test", string(b))
}
