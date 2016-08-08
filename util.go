package rex

import (
	"os"
	"math/rand"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
var letterRunesLen = len(letterRunes)

func GenRandFilename(length int) string {
	raw := make([]rune, length)

	for i := range raw {
		raw[i] = letterRunes[rand.Intn(letterRunesLen)]
	}

	return string(raw)
}

func PathExists(path string) (bool, error) {
	_, statErr := os.Stat(path)

	if statErr == nil {
		return true, nil
	} else if os.IsNotExist(statErr) {
		return false, nil
	}

	return false, statErr
}
