package beefyrelayer

import (
	"context"
	"encoding/hex"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sirupsen/logrus"
	gsrpcTypes "github.com/snowfork/go-substrate-rpc-client/v3/types"

	"github.com/snowfork/polkadot-ethereum/relayer/workers/beefyrelayer/store"
)

func (wr *BeefyEthereumWriter) LogBeefyFixtureDataAll(msg store.CompleteSignatureCommitmentMessage, info store.BeefyRelayInfo) {

	var latestMMRProof gsrpcTypes.GenerateMMRProofResponse
	gsrpcTypes.DecodeFromBytes(info.SerializedLatestMMRProof, &latestMMRProof)

	var hasher Keccak256

	bytesEncodedMMRLeaf, _ := gsrpcTypes.EncodeToBytes(msg.LatestMMRLeaf)

	// Leaf is double encoded
	hexEncodedLeaf, _ := gsrpcTypes.EncodeToHexString(bytesEncodedMMRLeaf)
	bytesEncodedLeaf, _ := gsrpcTypes.EncodeToBytes(bytesEncodedMMRLeaf)

	hashedLeaf := "0x" + hex.EncodeToString(hasher.Hash(bytesEncodedLeaf))

	parachainHeadsRootHex, _ := gsrpcTypes.EncodeToHexString(msg.LatestMMRLeaf.ParachainHeadsRoot)
	nextAuthoritySetRootHex, _ := gsrpcTypes.EncodeToHexString(msg.LatestMMRLeaf.NextAuthoritySetRoot)
	parentHashHex, _ := gsrpcTypes.EncodeToHexString(msg.LatestMMRLeaf.ParentHash)
	payloadHex, _ := gsrpcTypes.EncodeToHexString(msg.Commitment.Payload)

	var mmrProofItems []string
	for _, item := range msg.MMRProofItems {
		hex := "0x" + hex.EncodeToString(item[:])
		mmrProofItems = append(mmrProofItems, hex)
	}

	var signatures []string
	for _, item := range msg.Signatures {
		hex := hex.EncodeToString(item)
		signatures = append(signatures, "0x"+hex)
	}

	var pubKeys []string
	for _, item := range msg.ValidatorPublicKeys {
		hex := item.Hex()
		pubKeys = append(pubKeys, hex)
	}

	var pubKeyMerkleProofs [][]string
	for _, pubkeyProof := range msg.ValidatorPublicKeyMerkleProofs {
		var pubkeyProofS []string
		for _, item := range pubkeyProof {
			hex := "0x" + hex.EncodeToString(item[:])
			pubkeyProofS = append(pubkeyProofS, hex)
		}
		pubKeyMerkleProofs = append(pubKeyMerkleProofs, pubkeyProofS)
	}

	wr.log.WithFields(logrus.Fields{
		"msg.Commitment.BlockNumber":         msg.Commitment.BlockNumber,
		"msg.Commitment.Payload":             payloadHex,
		"msg.Commitment.ValidatorSetId":      msg.Commitment.ValidatorSetId,
		"msg.Signatures":                     signatures,
		"msg.ValidatorPositions":             msg.ValidatorPositions,
		"msg.ValidatorPublicKeys":            pubKeys,
		"msg.ValidatorPublicKeyMerkleProofs": pubKeyMerkleProofs,
		"LatestMMRLeaf.ParentNumber":         msg.LatestMMRLeaf.ParentNumber,
		"LatestMMRLeaf.ParentHash":           parentHashHex,
		"LatestMMRLeaf.ParachainHeadsRoot":   parachainHeadsRootHex,
		"LatestMMRLeaf.NextAuthoritySetId":   msg.LatestMMRLeaf.NextAuthoritySetId,
		"LatestMMRLeaf.NextAuthoritySetLen":  msg.LatestMMRLeaf.NextAuthoritySetLen,
		"LatestMMRLeaf.NextAuthoritySetRoot": nextAuthoritySetRootHex,
		"hexEncodedLeaf":                     hexEncodedLeaf,
		"hashedLeaf":                         hashedLeaf,
		"mmrProofItems":                      mmrProofItems,
	}).Info("Complete Signature Commitment transaction submitted")
}

// Keccak256 is the Keccak256 hashing method
type Keccak256 struct{}

// New creates a new Keccak256 hashing method
func New() *Keccak256 {
	return &Keccak256{}
}

// Hash generates a Keccak256 hash from a byte array
func (h *Keccak256) Hash(data []byte) []byte {
	hash := crypto.Keccak256(data)
	return hash[:]
}

func (wr *BeefyEthereumWriter) GetFailingMessage(client ethclient.Client, hash common.Hash) (string, error) {
	tx, _, err := client.TransactionByHash(context.Background(), hash)
	if err != nil {
		return "", err
	}

	from, err := types.Sender(types.NewEIP155Signer(tx.ChainId()), tx)
	if err != nil {
		return "", err
	}

	msg := ethereum.CallMsg{
		From:     from,
		To:       tx.To(),
		Gas:      tx.Gas(),
		GasPrice: tx.GasPrice(),
		Value:    tx.Value(),
		Data:     tx.Data(),
	}

	wr.log.WithFields(logrus.Fields{
		"From":     from,
		"To":       tx.To(),
		"Gas":      tx.Gas(),
		"GasPrice": tx.GasPrice(),
		"Value":    tx.Value(),
		"Data":     hex.EncodeToString(tx.Data()),
	}).Info("Call info")

	// The logger does a test call to the actual contract to check for any revert message and log it, as well
	// as logging the call info. This is because the golang client can sometimes supress the log message and so
	// it can be helpful to use the call info to do the same call in Truffle/Web3js to get better logs.
	res, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return "", err
	}

	return string(res), nil
}
