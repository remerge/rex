package rex

import (
	"bytes"
	"encoding/gob"
)

// GobEncode is a convenience function to return the gob representation
// of the given value. If this value implements an interface, it needs
// to be registered before GobEncode can be used.
func GobEncode(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(&v)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode is a convenience function to return the unmarshaled value
// of the given byte slice. If the value implements an interface, it
// needs to be registered before GobEncode can be used.
func GobDecode(b []byte) (interface{}, error) {
	var result interface{}
	err := gob.NewDecoder(bytes.NewBuffer(b)).Decode(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
