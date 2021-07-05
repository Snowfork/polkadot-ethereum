package store

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/snowfork/go-substrate-rpc-client/v3/types"
	"github.com/snowfork/polkadot-ethereum/relayer/contracts/beefylightclient"
	merkletree "github.com/wealdtech/go-merkletree"
	"golang.org/x/crypto/blake2b"
)

type NewSignatureCommitmentMessage struct {
	CommitmentHash                [32]byte
	ValidatorClaimsBitfield       []*big.Int
	ValidatorSignatureCommitment  []byte
	ValidatorPosition             *big.Int
	ValidatorPublicKey            common.Address
	ValidatorPublicKeyMerkleProof [][32]byte
}

type CompleteSignatureCommitmentMessage struct {
	ID                             *big.Int
	Commitment                     beefylightclient.BeefyLightClientCommitment
	Signatures                     [][]byte
	ValidatorPositions             []*big.Int
	ValidatorPublicKeys            []common.Address
	ValidatorPublicKeyMerkleProofs [][][32]byte
	LatestMMRLeaf                  beefylightclient.BeefyLightClientBeefyMMRLeaf
	MMRProofItems                  [][32]byte
}

type BeefyJustification struct {
	ValidatorPublicKeys []string
	SignedCommitment    SignedCommitment
}

func NewBeefyJustification(validatorPublicKeys []string, signedCommitment SignedCommitment) BeefyJustification {
	return BeefyJustification{
		ValidatorPublicKeys: validatorPublicKeys,
		SignedCommitment:    signedCommitment,
	}
}

func (b *BeefyJustification) BuildNewSignatureCommitmentMessage(valAddrIndex int, initialBitfield []*big.Int) (NewSignatureCommitmentMessage, error) {
	commitmentHash := blake2b.Sum256(b.SignedCommitment.Commitment.Bytes())

	sig0ProofContents, err := b.GenerateMerkleProofOffchain(valAddrIndex)
	if err != nil {
		return NewSignatureCommitmentMessage{}, err
	}

	sigValEthereum := BeefySigToEthSig(b.SignedCommitment.Signatures[valAddrIndex].Value)

	msg := NewSignatureCommitmentMessage{
		CommitmentHash:                commitmentHash,
		ValidatorClaimsBitfield:       initialBitfield,
		ValidatorSignatureCommitment:  sigValEthereum,
		ValidatorPublicKey:            b.ValidatorPublicKeys[valAddrIndex],
		ValidatorPosition:             big.NewInt(int64(valAddrIndex)),
		ValidatorPublicKeyMerkleProof: sig0ProofContents,
	}

	return msg, nil
}

func BeefySigToEthSig(beefySig BeefySignature) []byte {
	// Update signature format (Polkadot uses recovery IDs 0 or 1, Eth uses 27 or 28, so we need to add 27)
	// Split signature into r, s, v and add 27 to v
	sigValrs := beefySig[:64]
	sigValv := beefySig[64]
	sigValvAdded := byte(uint8(sigValv) + 27)
	sigValEthereum := append(sigValrs, sigValvAdded)

	return sigValEthereum
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

func (b *BeefyJustification) GenerateMerkleProofOffchain(valAddrIndex int) ([][32]byte, error) {
	// Hash validator addresses for leaf input data
	beefyTreeData := make([][]byte, len(b.ValidatorAddresses))
	for i, valAddr := range b.ValidatorAddresses {
		beefyTreeData[i] = valAddr.Bytes()
	}

	// Create the tree
	beefyMerkleTree, err := merkletree.NewUsing(beefyTreeData, &Keccak256{}, nil)
	if err != nil {
		return [][32]byte{}, err
	}

	// Generate Merkle Proof for validator at index valAddrIndex
	sigProof, err := beefyMerkleTree.GenerateProof(beefyTreeData[valAddrIndex])
	if err != nil {
		return [][32]byte{}, err
	}

	// Verify the proof
	root := beefyMerkleTree.Root()
	verified, err := merkletree.VerifyProofUsing(beefyTreeData[valAddrIndex], sigProof, root, &Keccak256{}, nil)
	if err != nil {
		return [][32]byte{}, err
	}
	if !verified {
		return [][32]byte{}, fmt.Errorf("failed to verify proof")
	}

	sigProofContents := make([][32]byte, len(sigProof.Hashes))
	for i, hash := range sigProof.Hashes {
		var hash32Byte [32]byte
		copy(hash32Byte[:], hash)
		sigProofContents[i] = hash32Byte
	}

	return sigProofContents, nil
}

func (b *BeefyJustification) BuildCompleteSignatureCommitmentMessage(info BeefyRelayInfo, bitfield string) (CompleteSignatureCommitmentMessage, error) {
	validationDataID := big.NewInt(int64(info.ContractID))

	validatorPositions := []*big.Int{}
	for i := 0; i < len(bitfield); i++ {
		bit := bitfield[i : i+1]
		if bit == "1" {
			validatorPositions = append(validatorPositions, big.NewInt(int64(i)))
		}
	}

	signatures := [][]byte{}
	validatorPublicKeys := []common.Address{}
	validatorPublicKeyMerkleProofs := [][][32]byte{}
	for _, validatorPosition := range validatorPositions {
		beefySig := b.SignedCommitment.Signatures[validatorPosition.Int64()].Value
		ethSig := BeefySigToEthSig(beefySig)
		signatures = append(signatures, ethSig)

		pubKey := b.ValidatorAddresses[validatorPosition.Int64()]
		validatorPublicKeys = append(validatorPublicKeys, pubKey)

		merkleProof, err := b.GenerateMerkleProofOffchain(int(validatorPosition.Int64()))
		if err != nil {
			return CompleteSignatureCommitmentMessage{}, err
		}

		validatorPublicKeyMerkleProofs = append(validatorPublicKeyMerkleProofs, merkleProof)
	}

	commitment := beefylightclient.BeefyLightClientCommitment{
		Payload:        b.SignedCommitment.Commitment.Payload,
		BlockNumber:    uint64(b.SignedCommitment.Commitment.BlockNumber),
		ValidatorSetId: uint32(b.SignedCommitment.Commitment.ValidatorSetID),
	}

	var latestMMRProof types.GenerateMMRProofResponse
	err := types.DecodeFromBytes(info.SerializedLatestMMRProof, &latestMMRProof)
	if err != nil {
		return CompleteSignatureCommitmentMessage{}, err
	}

	latestMMRLeaf := beefylightclient.BeefyLightClientBeefyMMRLeaf{
		ParentNumber:         uint32(latestMMRProof.Leaf.ParentNumberAndHash.ParentNumber),
		ParentHash:           latestMMRProof.Leaf.ParentNumberAndHash.Hash,
		ParachainHeadsRoot:   latestMMRProof.Leaf.ParachainHeads,
		NextAuthoritySetId:   uint64(latestMMRProof.Leaf.BeefyNextAuthoritySet.ID),
		NextAuthoritySetLen:  uint32(latestMMRProof.Leaf.BeefyNextAuthoritySet.Len),
		NextAuthoritySetRoot: latestMMRProof.Leaf.BeefyNextAuthoritySet.Root,
	}
	mmrProofItems := [][32]byte{}
	for _, mmrProofItem := range latestMMRProof.Proof.Items {
		mmrProofItems = append(mmrProofItems, mmrProofItem)
	}

	msg := CompleteSignatureCommitmentMessage{
		ID:                             validationDataID,
		Commitment:                     commitment,
		Signatures:                     signatures,
		ValidatorPositions:             validatorPositions,
		ValidatorPublicKeys:            validatorPublicKeys,
		ValidatorPublicKeyMerkleProofs: validatorPublicKeyMerkleProofs,
		LatestMMRLeaf:                  latestMMRLeaf,
		MMRProofItems:                  mmrProofItems,
	}
	return msg, nil
}
