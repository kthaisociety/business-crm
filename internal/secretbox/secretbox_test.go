package secretbox

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var key = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))

func TestSealedSecretsOpenOnlyWithTheirKey(t *testing.T) {
	box, err := New(key)
	require.NoError(t, err)

	sealed := box.Seal([]byte("refresh-token"))
	assert.NotContains(t, string(sealed), "refresh-token")
	assert.NotEqual(t, sealed, box.Seal([]byte("refresh-token")), "a fresh nonce each time")

	opened, err := box.Open(sealed)
	require.NoError(t, err)
	assert.Equal(t, "refresh-token", string(opened))

	other, _ := New(base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{8}, 32)))
	_, err = other.Open(sealed)
	assert.Error(t, err)
	sealed[len(sealed)-1] ^= 1
	_, err = box.Open(sealed)
	assert.Error(t, err, "tampering is caught")
}

func TestKeysMustBe32Bytes(t *testing.T) {
	for _, bad := range []string{"", "not base64!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		_, err := New(bad)
		assert.Error(t, err, bad)
	}
}
