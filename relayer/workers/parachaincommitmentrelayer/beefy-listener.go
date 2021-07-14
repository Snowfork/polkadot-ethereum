package parachaincommitmentrelayer

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/snowfork/go-substrate-rpc-client/v3/types"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/ethereum"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/parachain"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/relaychain"
	"github.com/snowfork/polkadot-ethereum/relayer/contracts/beefylightclient"
)

type BeefyListener struct {
	ethereumConfig      *ethereum.Config
	ethereumConn        *ethereum.Connection
	beefyLightClient    *beefylightclient.Contract
	relaychainConn      *relaychain.Connection
	parachainConnection *parachain.Connection
	paraID              uint32
	messages            chan<- MessagePackage
	log                 *logrus.Entry
}

func NewBeefyListener(
	ethereumConfig *ethereum.Config,
	ethereumConn *ethereum.Connection,
	relaychainConn *relaychain.Connection,
	parachainConnection *parachain.Connection,
	messages chan<- MessagePackage,
	log *logrus.Entry) *BeefyListener {
	return &BeefyListener{
		ethereumConfig:      ethereumConfig,
		ethereumConn:        ethereumConn,
		relaychainConn:      relaychainConn,
		parachainConnection: parachainConnection,
		messages:            messages,
		log:                 log,
	}
}

func (li *BeefyListener) Start(ctx context.Context, eg *errgroup.Group) error {

	// Set up light client bridge contract
	beefyLightClientContract, err := beefylightclient.NewContract(common.HexToAddress(li.ethereumConfig.BeefyLightClient), li.ethereumConn.GetClient())
	if err != nil {
		return err
	}
	li.beefyLightClient = beefyLightClientContract

	// Fetch ParaId
	storageKeyForParaID, err := types.CreateStorageKey(li.parachainConnection.GetMetadata(), "ParachainInfo", "ParachainId", nil, nil)
	if err != nil {
		return err
	}

	var paraID uint32
	ok, err := li.parachainConnection.GetAPI().RPC.State.GetStorageLatest(storageKeyForParaID, &paraID)
	if err != nil {
		li.log.WithError(err).Error("Failed to get para id for snowbridge")
		return err
	}
	if !ok {
		li.log.Error("Expected parachain but chain does not provide a parachain ID")
		return fmt.Errorf("invalid parachain")
	}

	li.log.WithField("paraId", paraID).Info("Fetched parachain id")
	li.paraID = paraID

	eg.Go(func() error {

		verifiedBeefyBlockNumber, verifiedBeefyBlockHash, err := li.fetchLatestVerifiedBeefyBlock(ctx)
		if err != nil {
			li.log.WithError(err).Error("Failed to get latest relay chain block number and hash")
			return err
		}

		verifiedParaBlockNumber, err := li.relaychainConn.FetchLatestFinalizedParaBlockNumber(
			verifiedBeefyBlockHash, paraID)
		if err != nil {
			li.log.WithError(err).Error("Failed to get latest finalized para block number from relay chain")
			return err
		}

		verifiedParaBlockHash, err := li.parachainConnection.GetAPI().RPC.Chain.GetBlockHash(verifiedParaBlockNumber)
		if err != nil {
			li.log.WithError(err).Error("Failed to get latest finalized para block hash")
			return err
		}

		messagePackages, err := li.buildMissedMessagePackages(ctx, verifiedBeefyBlockNumber, verifiedParaBlockNumber, verifiedParaBlockHash)
		if err != nil {
			li.log.WithError(err).Error("Failed to build missed message package")
			return err
		}

		li.emitMessagePackages(messagePackages)

		err = li.subBeefyJustifications(ctx)
		return err
	})

	return nil
}

func (li *BeefyListener) onDone(ctx context.Context) error {
	li.log.Info("Shutting down listener...")
	if li.messages != nil {
		close(li.messages)
	}
	return ctx.Err()
}

func (li *BeefyListener) subBeefyJustifications(ctx context.Context) error {
	headers := make(chan *gethTypes.Header, 5)

	sub, err := li.ethereumConn.GetClient().SubscribeNewHead(ctx, headers)
	if err != nil {
		li.log.WithError(err).Error("Error creating ethereum header subscription")
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return li.onDone(ctx)
		case err := <-sub.Err():
			li.log.WithError(err).Error("Error with ethereum header subscription")
			return err
		case gethheader := <-headers:
			// Query LightClientBridge contract's ContractNewMMRRoot events
			blockNumber := gethheader.Number.Uint64()
			var beefyLightClientEvents []*beefylightclient.ContractNewMMRRoot

			contractEvents, err := li.queryBeefyLightClientEvents(ctx, blockNumber, &blockNumber)
			if err != nil {
				li.log.WithError(err).Error("Failure fetching event logs")
				return err
			}
			beefyLightClientEvents = append(beefyLightClientEvents, contractEvents...)

			if len(beefyLightClientEvents) > 0 {
				li.log.Info(fmt.Sprintf("Found %d BeefyLightClient ContractNewMMRRoot events on block %d", len(beefyLightClientEvents), blockNumber))
			}
			li.processBeefyLightClientEvents(ctx, beefyLightClientEvents)
		}
	}
}

// processLightClientEvents matches events to BEEFY commitment info by transaction hash
func (li *BeefyListener) processBeefyLightClientEvents(ctx context.Context, events []*beefylightclient.ContractNewMMRRoot) error {
	for _, event := range events {

		beefyBlockNumber := event.BlockNumber

		li.log.WithFields(logrus.Fields{
			"beefyBlockNumber":    beefyBlockNumber,
			"ethereumBlockNumber": event.Raw.BlockNumber,
			"ethereumTxHash":      event.Raw.TxHash.Hex(),
		}).Info("Witnessed a new MMRRoot event")

		li.log.WithField("beefyBlockNumber", beefyBlockNumber).Info("Getting hash for relay chain block")
		relayBlockHash, err := li.relaychainConn.GetAPI().RPC.Chain.GetBlockHash(uint64(beefyBlockNumber))
		if err != nil {
			li.log.WithError(err).Error("Failed to get block hash")
			return err
		}
		li.log.WithField("relayBlockHash", relayBlockHash.Hex()).Info("Got relay chain blockhash")

		verifiedParaBlockNumber, err := li.relaychainConn.FetchLatestFinalizedParaBlockNumber(
			relayBlockHash, li.paraID)
		if err != nil {
			li.log.WithError(err).Error("Failed to get latest finalized para block number from relay chain")
			return err
		}
		verifiedParaBlockHash, err := li.parachainConnection.GetAPI().RPC.Chain.GetBlockHash(verifiedParaBlockNumber)
		if err != nil {
			li.log.WithError(err).Error("Failed to get latest finalized para block hash")
			return err
		}

		messagePackages, err := li.buildMissedMessagePackages(ctx, beefyBlockNumber, verifiedParaBlockNumber, verifiedParaBlockHash)
		if err != nil {
			li.log.WithError(err).Error("Failed to build missed message packages")
			return err
		}

		li.emitMessagePackages(messagePackages)

	}
	return nil
}

func (li *BeefyListener) emitMessagePackages(packages []MessagePackage) {
	for _, messagePackage := range packages {
		li.log.WithFields(logrus.Fields{
			"channelID":             messagePackage.channelID,
			"commitmentHash":        messagePackage.commitmentHash,
			"commitmentData":        messagePackage.commitmentData,
			"ourParaHeadProof":      messagePackage.paraHeadProof,
			"ourParaHeadProofPos":   messagePackage.paraHeadProofPos,
			"ourParaHeadProofWidth": messagePackage.paraHeadProofWidth,
			"mmrProof":              messagePackage.mmrProof,
		}).Info("Beefy Listener emitted new message package")

		li.messages <- messagePackage
	}
}

// queryBeefyLightClientEvents queries ContractNewMMRRoot events from the BeefyLightClient contract
func (li *BeefyListener) queryBeefyLightClientEvents(ctx context.Context, start uint64,
	end *uint64) ([]*beefylightclient.ContractNewMMRRoot, error) {
	var events []*beefylightclient.ContractNewMMRRoot
	filterOps := bind.FilterOpts{Start: start, End: end, Context: ctx}

	iter, err := li.beefyLightClient.FilterNewMMRRoot(&filterOps)
	if err != nil {
		return nil, err
	}

	for {
		more := iter.Next()
		if !more {
			err = iter.Error()
			if err != nil {
				return nil, err
			}
			break
		}

		events = append(events, iter.Event)
	}

	return events, nil
}
