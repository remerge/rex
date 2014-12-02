package rex

import (
	"crypto/sha1"
	"io"
	"os"
)

func SHA1(data []byte) []byte {
	hasher := sha1.New()
	hasher.Write(data)
	return hasher.Sum(nil)
}

func SHA1S(s string) []byte {
	return SHA1([]byte(s))
}

func SHA1F(filename string) []byte {
	file, err := os.Open(filename)
	if err != nil {
		return nil
	}

	defer file.Close()

	hasher := sha1.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return nil
	}

	return hasher.Sum(nil)
}
