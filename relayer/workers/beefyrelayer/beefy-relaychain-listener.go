package beefyrelayer

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/sirupsen/logrus"
	"github.com/snowfork/go-substrate-rpc-client/v3/types"
	"golang.org/x/sync/errgroup"

	"github.com/snowfork/polkadot-ethereum/relayer/chain/relaychain"
	"github.com/snowfork/polkadot-ethereum/relayer/substrate"
	"github.com/snowfork/polkadot-ethereum/relayer/workers/beefyrelayer/store"
)

type BeefyRelaychainListener struct {
	relaychainConfig *relaychain.Config
	relaychainConn   *relaychain.Connection
	beefyMessages    chan<- store.BeefyRelayInfo
	log              *logrus.Entry
}

func NewBeefyRelaychainListener(relaychainConfig *relaychain.Config,
	relaychainConn *relaychain.Connection, beefyMessages chan<- store.BeefyRelayInfo,
	log *logrus.Entry) *BeefyRelaychainListener {
	return &BeefyRelaychainListener{
		relaychainConfig: relaychainConfig,
		relaychainConn:   relaychainConn,
		beefyMessages:    beefyMessages,
		log:              log,
	}
}

func (li *BeefyRelaychainListener) Start(ctx context.Context, eg *errgroup.Group) error {

	eg.Go(func() error {
		return li.subBeefyJustifications(ctx)
	})

	return nil
}

func (li *BeefyRelaychainListener) onDone(ctx context.Context) error {
	li.log.Info("Shutting down listener...")
	if li.beefyMessages != nil {
		close(li.beefyMessages)
	}
	return ctx.Err()
}

func (li *BeefyRelaychainListener) subBeefyJustifications(ctx context.Context) error {
	ch := make(chan interface{})

	sub, err := li.relaychainConn.GetAPI().Client.Subscribe(context.Background(), "beefy", "subscribeJustifications", "unsubscribeJustifications", "justifications", ch)
	if err != nil {
		panic(err)
	}
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return li.onDone(ctx)
		case msg := <-ch:

			signedCommitment := &store.SignedCommitment{}
			err := types.DecodeFromHexString(msg.(string), signedCommitment)
			if err != nil {
				li.log.WithError(err).Error("Failed to decode BEEFY commitment messages")
			}

			li.log.Info("Witnessed a new BEEFY commitment: \n", msg.(string))
			if len(signedCommitment.Signatures) == 0 {
				li.log.Info("BEEFY commitment has no signatures, skipping...")
				continue
			}

			signedCommitmentBytes, err := json.Marshal(signedCommitment)
			if err != nil {
				li.log.WithError(err).Error("Failed to marshal signed commitment:", signedCommitment)
				continue
			}

			blockNumber := uint64(signedCommitment.Commitment.BlockNumber)

			beefyAuthorities, err := li.getBeefyAuthorities(blockNumber)
			if err != nil {
				li.log.WithError(err).Error("Failed to get Beefy authorities from on-chain storage")
				return err
			}

			blockHash, err := li.relaychainConn.GetAPI().RPC.Chain.GetBlockHash(uint64(blockNumber))
			if err != nil {
				li.log.WithError(err).Error("Failed to get block hash")
			}
			li.log.WithField("blockHash", blockHash.Hex()).Info("Got next blockhash")

			latestMMRProof, err := li.relaychainConn.GetMMRLeafForBlock(blockNumber-1, blockHash)
			if err != nil {
				li.log.WithError(err).Error("Failed get MMR Leaf")
				return err
			}
			serializedProof, err := types.EncodeToBytes(latestMMRProof)
			if err != nil {
				li.log.WithError(err).Error("Failed to serialize MMR Proof")
				return err
			}

			info := store.BeefyRelayInfo{
				ValidatorAddresses:       beefyAuthorities,
				SignedCommitment:         signedCommitmentBytes,
				Status:                   store.CommitmentWitnessed,
				SerializedLatestMMRProof: serializedProof,
			}
			li.beefyMessages <- info
		}
	}
}

func (li *BeefyRelaychainListener) getBeefyAuthorities(blockNumber uint64) ([]string, error) {
	blockHash, err := li.relaychainConn.GetAPI().RPC.Chain.GetBlockHash(blockNumber)
	if err != nil {
		return nil, err
	}

	meta, err := li.relaychainConn.GetAPI().RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, err
	}

	storageKey, err := types.CreateStorageKey(meta, "Beefy", "Authorities", nil, nil)
	if err != nil {
		return nil, err
	}

	storageChangeSet, err := li.relaychainConn.GetAPI().RPC.State.QueryStorage([]types.StorageKey{storageKey}, blockHash, blockHash)
	if err != nil {
		return nil, err
	}

	authorities := substrate.Authorities{}
	for _, storageChange := range storageChangeSet {
		for _, keyValueOption := range storageChange.Changes {

			err = types.DecodeFromHexString(keyValueOption.StorageData.Hex(), &authorities)
			if err != nil {
				return nil, err
			}

		}
	}

	// Convert from beefy authorities to ethereum addresses
	var authorityPubkeys []string
	for _, authority := range authorities {
		pub, err := crypto.DecompressPubkey(authority[:])
		pubBytes := crypto.FromECDSAPub(pub)
		pubHex := hex.EncodeToString(pubBytes)
		if err != nil {
			return nil, err
		}
		authorityPubkeys = append(authorityPubkeys, pubHex)
	}

	return authorityPubkeys, nil
}

func DecompressedKeyToEthAddress(pub *ecdsa.PublicKey) common.Address {
	ethereumAddress := crypto.PubkeyToAddress(*pub)
	return ethereumAddress
}
