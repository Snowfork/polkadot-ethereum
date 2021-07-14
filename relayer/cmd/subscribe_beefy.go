package cmd

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/snowfork/go-substrate-rpc-client/v3/types"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/parachain"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/relaychain"
	"github.com/snowfork/polkadot-ethereum/relayer/core"
	"github.com/snowfork/polkadot-ethereum/relayer/workers/beefyrelayer/store"
	"github.com/spf13/cobra"
)

const OUR_PARACHAIN_ID = 200

func subBeefyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sub-beefy",
		Short:   "Subscribe beefy messages",
		Args:    cobra.ExactArgs(0),
		Example: "artemis-relay sub-beefy",
		RunE:    SubBeefyFn,
	}
	cmd.Flags().UintP(
		"para-id",
		"i",
		1000,
		"Parachain ID",
	)
	cmd.MarkFlagRequired("para-id")
	return cmd
}

func SubBeefyFn(cmd *cobra.Command, _ []string) error {
	paraID, err := cmd.Flags().GetUint32("para-id")
	if err != nil {
		return err
	}
	subBeefyJustifications(cmd.Context(), paraID)
	return nil
}

func subBeefyJustifications(ctx context.Context, paraID uint32) error {
	log.Info("Loading config")
	config, err := core.LoadConfig()
	if err != nil {
		log.Error(err)
		return err
	}

	log := log.WithField("script", "beefy")

	relaychainEndpoint := config.Relaychain.Endpoint
	relaychainConn := relaychain.NewConnection(relaychainEndpoint, log)
	err = relaychainConn.Connect(ctx)
	if err != nil {
		log.Error(err)
		return err
	}

	parachainEndpoint := config.Parachain.Endpoint
	parachainConn := parachain.NewConnection(parachainEndpoint, nil, log)
	err = parachainConn.Connect(ctx)
	if err != nil {
		log.Error(err)
		return err
	}

	ch := make(chan interface{})

	log.Info("Subscribing to beefy justifications")
	sub, err := relaychainConn.GetAPI().Client.Subscribe(context.Background(), "beefy", "subscribeJustifications", "unsubscribeJustifications", "justifications", ch)
	if err != nil {
		panic(err)
	}
	defer sub.Unsubscribe()

	for {
		select {
		case msg := <-ch:

			signedCommitment := &store.SignedCommitment{}
			err := types.DecodeFromHexString(msg.(string), signedCommitment)
			if err != nil {
				log.WithError(err).Error("Failed to decode BEEFY commitment messages")
			}

			blockNumber := signedCommitment.Commitment.BlockNumber

			log.WithField("commitmentBlockNumber", blockNumber).Info("Witnessed a new BEEFY commitment: \n")
			if len(signedCommitment.Signatures) == 0 {
				log.Info("BEEFY commitment has no signatures, skipping...")
				continue
			}
			log.WithField("blockNumber", blockNumber+1).Info("Getting hash for next block")
			nextBlockHash, err := relaychainConn.GetAPI().RPC.Chain.GetBlockHash(uint64(blockNumber + 1))
			if err != nil {
				log.WithError(err).Error("Failed to get block hash")
			}
			log.WithField("blockHash", nextBlockHash.Hex()).Info("Got blockhash")
			GetMMRLeafForBlock(uint64(blockNumber), nextBlockHash, relaychainConn)
			allParaheads, ourParahead := GetAllParaheads(nextBlockHash, relaychainConn, paraID)
			log.WithFields(logrus.Fields{
				"allParaheads": allParaheads,
				"ourParahead":  ourParahead,
			}).Info("Got all para heads")

			// TODO6 - Update all above code to make sure to check all new parachain blocks that have been added to the MMR
			// when there is a new beefy justification, not just the newest parachain block in the MMR
		}
	}
}

func GetAllParaheads(blockHash types.Hash, relaychainConn *relaychain.Connection, ourParachainID uint32) ([]types.Header, types.Header) {
	none := types.NewOptionU32Empty()
	encoded, err := types.EncodeToBytes(none)
	if err != nil {
		log.WithError(err).Error("Error")
	}

	baseParaHeadsStorageKey, err := types.CreateStorageKey(
		relaychainConn.GetMetadata(),
		"Paras",
		"Heads", encoded, nil)
	if err != nil {
		log.WithError(err).Error("Failed to create parachain header storage key")
	}

	//TODO fix this manual slice.
	// The above types.CreateStorageKey does not give the same base key as polkadotjs needs for getKeys.
	// It has some extra bytes.
	// maybe from the none u32 in golang being wrong, or maybe slightly off CreateStorageKey call? we slice it
	// here as a hack.
	actualBaseParaHeadsStorageKey := baseParaHeadsStorageKey[:32]
	log.WithField("actualBaseParaHeadsStorageKey", actualBaseParaHeadsStorageKey.Hex()).Info("actualBaseParaHeadsStorageKey")

	keysResponse, err := relaychainConn.GetAPI().RPC.State.GetKeys(actualBaseParaHeadsStorageKey, blockHash)
	if err != nil {
		log.WithError(err).Error("Failed to get all parachain keys")
	}

	headersResponse, err := relaychainConn.GetAPI().RPC.State.QueryStorage(keysResponse, blockHash, blockHash)
	if err != nil {
		log.WithError(err).Error("Failed to get all parachain headers")
	}

	log.Info("Got all parachain headers")
	var headers []types.Header
	var ourParachainHeader types.Header
	for _, headerResponse := range headersResponse {
		for _, change := range headerResponse.Changes {

			// TODO fix this manual slice with a proper type decode. only the last few bytes are for the ParaId,
			// not sure what the early ones are for.
			key := change.StorageKey[40:]
			var parachainID types.U32
			if err := types.DecodeFromBytes(key, &parachainID); err != nil {
				log.WithError(err).Error("Failed to decode parachain ID")
			}

			log.WithField("parachainId", parachainID).Info("Decoding header for parachain")
			var encodableOpaqueHeader types.Bytes
			if err := types.DecodeFromBytes(change.StorageData, &encodableOpaqueHeader); err != nil {
				log.WithError(err).Error("Failed to decode MMREncodableOpaqueLeaf")
			}

			var header types.Header
			if err := types.DecodeFromBytes(encodableOpaqueHeader, &header); err != nil {
				log.WithError(err).Error("Failed to decode Header")
			}
			log.WithFields(logrus.Fields{
				"headerBytes":           fmt.Sprintf("%#x", encodableOpaqueHeader),
				"header.ParentHash":     header.ParentHash.Hex(),
				"header.Number":         header.Number,
				"header.StateRoot":      header.StateRoot.Hex(),
				"header.ExtrinsicsRoot": header.ExtrinsicsRoot.Hex(),
				"header.Digest":         header.Digest,
				"parachainId":           parachainID,
			}).Info("Decoded header for parachain")
			headers = append(headers, header)

			if parachainID == types.U32(ourParachainID) {
				ourParachainHeader = header
			}
		}
	}
	return headers, ourParachainHeader
}

func GetMMRLeafForBlock(blockNumber uint64, blockHash types.Hash, relaychainConn *relaychain.Connection) {
	log.WithFields(logrus.Fields{
		"blockNumber": blockNumber,
		"blockHash":   blockHash.Hex(),
	}).Info("Getting MMR Leaf for block...")
	proofResponse, err := relaychainConn.GetAPI().RPC.MMR.GenerateProof(blockNumber, blockHash)
	if err != nil {
		log.WithError(err).Error("Failed to generate mmr proof")
	}

	log.WithFields(logrus.Fields{
		"BlockHash":                       proofResponse.BlockHash.Hex(),
		"Leaf.ParentNumber":               proofResponse.Leaf.ParentNumberAndHash.ParentNumber,
		"Leaf.Hash":                       proofResponse.Leaf.ParentNumberAndHash.Hash.Hex(),
		"Leaf.ParachainHeads":             proofResponse.Leaf.ParachainHeads.Hex(),
		"Leaf.BeefyNextAuthoritySet.ID":   proofResponse.Leaf.BeefyNextAuthoritySet.ID,
		"Leaf.BeefyNextAuthoritySet.Len":  proofResponse.Leaf.BeefyNextAuthoritySet.Len,
		"Leaf.BeefyNextAuthoritySet.Root": proofResponse.Leaf.BeefyNextAuthoritySet.Root.Hex(),
		"Proof.LeafIndex":                 proofResponse.Proof.LeafIndex,
		"Proof.LeafCount":                 proofResponse.Proof.LeafCount,
	}).Info("Generated MMR Proof")
}
