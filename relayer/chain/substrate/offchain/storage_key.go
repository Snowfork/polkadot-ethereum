package offchain

import (
	"bytes"

	"github.com/snowfork/go-substrate-rpc-client/v2/scale"
	"github.com/snowfork/go-substrate-rpc-client/v2/types"
)

var IndexingPrefix = []byte("commitment")

func MakeStorageKey(hash types.H256) ([]byte, error) {
	var buffer = bytes.Buffer{}
	encoder := scale.NewEncoder(&buffer)

	err := encoder.Encode(IndexingPrefix)
	if err != nil {
		return nil, err
	}

	err = encoder.Encode(hash)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
