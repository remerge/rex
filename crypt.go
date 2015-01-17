package rex

import (
	"crypto/hmac"
	"crypto/sha1"
	"fmt"
)

func DecryptHmacXorWithIntegrity(message, encrypt_key, integrity_key []byte) ([]byte, error) {
	size := len(message)
	initialization_vector := message[0:16]
	ciphertext := message[16 : size-4]
	integrity_signature := message[size-4 : size]

	mac := hmac.New(sha1.New, encrypt_key)
	mac.Write(initialization_vector)
	pad := mac.Sum(nil)

	unciphered := make([]byte, size-20)
	for i := 0; i < size-20; i++ {
		unciphered[i] = pad[i] ^ ciphertext[i]
	}

	mac = hmac.New(sha1.New, integrity_key)
	mac.Write(unciphered)
	mac.Write(initialization_vector)
	signature := mac.Sum(nil)[0:4]

	// check signature
	for i := 0; i < 4; i++ {
		if signature[i] != integrity_signature[i] {
			return nil, fmt.Errorf("signature %#v does not match integrity %#v", signature, integrity_signature)
		}
	}

	return unciphered, nil
}
