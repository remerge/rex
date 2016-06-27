package kafka

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"io"
	"strconv"
)

type OffsetMap map[int32]int64

func (w OffsetMap) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})
	for k, v := range w {
		m[strconv.Itoa(int(k))] = v
	}
	return json.Marshal(m)
}

func ReadOffsetMapFrom(r io.Reader) (om OffsetMap, err error) {
	decoder := gob.NewDecoder(r)
	err = decoder.Decode(&om)
	return om, err
}

func (om OffsetMap) WriteTo(w io.Writer) (int64, error) {
	var b bytes.Buffer
	encoder := gob.NewEncoder(&b)
	err := encoder.Encode(&om)
	return int64(b.Len()), err
}

func (om OffsetMap) Bytes() ([]byte, error) {
	var b bytes.Buffer
	encoder := gob.NewEncoder(&b)
	if err := encoder.Encode(&om); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func (om OffsetMap) SetOffset(partition int32, offset int64) {
	om[partition] = offset
}

func (om OffsetMap) Partitions() []int32 {
	keys := make([]int32, len(om))
	i := 0
	for k := range om {
		keys[i] = k
		i++
	}
	return keys
}
