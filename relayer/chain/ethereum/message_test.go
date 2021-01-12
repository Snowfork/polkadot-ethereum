package ethereum_test

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	gethTrie "github.com/ethereum/go-ethereum/trie"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/snowfork/go-substrate-rpc-client/v2/scale"
	"github.com/snowfork/polkadot-ethereum/relayer/chain"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/ethereum"
	"github.com/stretchr/testify/assert"
)

func encodeToBytes(value interface{}) ([]byte, error) {
	var buffer = bytes.Buffer{}
	err := scale.NewEncoder(&buffer).Encode(value)
	if err != nil {
		return buffer.Bytes(), err
	}
	return buffer.Bytes(), nil
}

func decodeFromBytes(bz []byte, target interface{}) error {
	return scale.NewDecoder(bytes.NewReader(bz)).Decode(target)
}

type TestProof ethereum.Proof

// For interface gethTrie.KeyValueReader
func (tp *TestProof) Get(key []byte) ([]byte, error) {
	for i, k := range tp.Keys {
		if bytes.Equal(k, key) {
			return tp.Values[i], nil
		}
	}
	return nil, fmt.Errorf("Value for key %s does not exist", key)
}

// For interface gethTrie.KeyValueReader
func (tp *TestProof) Has(key []byte) (bool, error) {
	_, err := tp.Get(key)
	return err == nil, nil
}

func TestMessage_EncodeDecode(t *testing.T) {

	input := ethereum.Message{
		Data: []byte{0, 1, 2},
		VerificationInput: ethereum.VerificationInput{
			IsBasic: true,
			AsBasic: ethereum.VerificationBasic{
				BlockNumber: 938,
				EventIndex:  4,
			},
		},
	}

	encoded, err := encodeToBytes(input)
	if err != nil {
		panic(err)
	}

	fmt.Println("length: ", len(encoded))

	var decoded ethereum.Message
	err = decodeFromBytes(encoded, &decoded)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, input, decoded, "The two messages should be the same")
}

func TestMessage_Proof(t *testing.T) {
	block := block11408438()
	receipts := receipts11408438()
	// We'll prove inclusion for this event by proving inclusion for
	// the encapsulating receipt
	event5_5 := receipts[5].Logs[5]
	receipt5Encoded, err := rlp.EncodeToBytes(receipts[5])
	if err != nil {
		panic(err)
	}

	// Construct Merkle Patricia Trie for receipts
	keybuf := new(bytes.Buffer)
	receiptTrie := new(gethTrie.Trie)
	for i := 0; i < receipts.Len(); i++ {
		keybuf.Reset()
		rlp.Encode(keybuf, uint(i))
		receiptTrie.Update(keybuf.Bytes(), receipts.GetRlp(i))
	}
	if receiptTrie.Hash() != block.ReceiptHash() {
		panic("Receipt trie does not match block receipt hash")
	}

	logger, _ := test.NewNullLogger()
	msg, err := ethereum.MakeMessageFromEvent(event5_5, receiptTrie, logger.WithField("test", "ing"))
	assert.Nil(t, err)
	assert.NotNil(t, msg)

	// Retrieve the encapsulating receipt from the proof using the payload fields
	assert.True(t, msg.VerificationInput.IsReceiptProof)
	proof := msg.VerificationInput.AsReceiptProof
	assert.Equal(t, block.Hash().Hex(), proof.BlockHash.Hex())
	key, err := rlp.EncodeToBytes(uint(proof.TxIndex))
	if err != nil {
		panic(err)
	}
	proofNodes := TestProof(*proof.Proof)
	provenReceipt, err := gethTrie.VerifyProof(block.ReceiptHash(), key, &proofNodes)
	assert.Nil(t, err)
	assert.Equal(t, provenReceipt, receipt5Encoded)
}

func TestMessage_Chunking(t *testing.T) {
	address1 := gethCommon.BigToAddress(big.NewInt(1))
	address2 := gethCommon.BigToAddress(big.NewInt(2))
	address3 := gethCommon.BigToAddress(big.NewInt(3))
	address4 := gethCommon.BigToAddress(big.NewInt(4))
	m := ethereum.Message{}
	messagesOriginal := map[gethCommon.Address][]*ethereum.Message{
		address1: {&m, &m, &m},
		address2: {&m},
		address3: {&m, &m},
		address4: {&m},
	}

	messages := make(map[gethCommon.Address][]*ethereum.Message)
	addToMessages := func(chunk []chain.Message) {
		for _, msg := range chunk {
			messages[msg.AppID] = append(messages[msg.AppID], msg.Payload.([]*ethereum.Message)...)
		}
	}

	nextChunk := ethereum.MakeMessageChunker(messagesOriginal, 2)
	chunk, hasMore := nextChunk()
	addToMessages(chunk)
	assert.True(t, hasMore)

	chunk, hasMore = nextChunk()
	addToMessages(chunk)
	assert.True(t, hasMore)

	chunk, hasMore = nextChunk()
	addToMessages(chunk)
	assert.True(t, hasMore)

	chunk, hasMore = nextChunk()
	addToMessages(chunk)
	assert.False(t, hasMore)

	assert.Equal(t, messagesOriginal, messages)
}
