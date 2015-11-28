package rex

import (
	"crypto/sha1"
	"io"
	"os"

	"github.com/remerge/rex/rollbar"
)

func SHA1(data []byte) []byte {
	hasher := sha1.New()
	_, err := hasher.Write(data)
	rollbar.Error(rollbar.WARN, err)
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

	defer func() {
		rollbar.Error(rollbar.WARN, file.Close())
	}()

	hasher := sha1.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return nil
	}

	return hasher.Sum(nil)
}
