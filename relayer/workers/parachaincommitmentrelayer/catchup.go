package parachaincommitmentrelayer

import (
	"context"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
	"github.com/snowfork/go-substrate-rpc-client/v2/rpc/offchain"
	"github.com/snowfork/go-substrate-rpc-client/v2/types"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/parachain"
	"github.com/snowfork/polkadot-ethereum/relayer/contracts/basic"
	"github.com/snowfork/polkadot-ethereum/relayer/contracts/incentivized"
	"github.com/snowfork/polkadot-ethereum/relayer/substrate"
	chainTypes "github.com/snowfork/polkadot-ethereum/relayer/substrate"
)

type ParaBlockWithDigest struct {
	BlockNumber         uint64
	DigestItemsWithData []DigestItemWithData
}

type ParaBlockWithProofs struct {
	Block            ParaBlockWithDigest
	MMRProofResponse types.GenerateMMRProofResponse
	Header           types.Header
	HeaderProof      string
}

type DigestItemWithData struct {
	DigestItem chainTypes.AuxiliaryDigestItem
	Data       types.StorageDataRaw
}

type MessagePackage struct {
	channelID      chainTypes.ChannelID
	commitmentHash types.H256
	commitmentData types.StorageDataRaw
	paraHead       types.Header
	paraHeadProof  string
	mmrProof       types.GenerateMMRProofResponse
}

// Catches up by searching for and relaying all missed commitments before the given para block
// This method implicitly assumes that relaychainBlock or some earlier relay chain block has
// already finalized the given para block
func (li *BeefyListener) buildMissedMessagePackets(
	ctx context.Context, relaychainBlock uint64, paraBlock uint64, paraHash types.Hash) (
	[]MessagePackage, error) {
	basicContract, err := basic.NewBasicInboundChannel(common.HexToAddress(
		li.ethereumConfig.Channels.Basic.Inbound),
		li.ethereumConn.GetClient(),
	)
	if err != nil {
		return nil, err
	}

	incentivizedContract, err := incentivized.NewIncentivizedInboundChannel(common.HexToAddress(
		li.ethereumConfig.Channels.Incentivized.Inbound),
		li.ethereumConn.GetClient(),
	)
	if err != nil {
		return nil, err
	}

	options := bind.CallOpts{
		Pending: true,
		Context: ctx,
	}

	ethBasicNonce, err := basicContract.Nonce(&options)
	if err != nil {
		return nil, err
	}
	li.log.WithFields(logrus.Fields{
		"nonce": ethBasicNonce,
	}).Info("Checked latest nonce delivered to ethereum basic channel")

	ethIncentivizedNonce, err := incentivizedContract.Nonce(&options)
	if err != nil {
		return nil, err
	}
	li.log.WithFields(logrus.Fields{
		"nonce": ethIncentivizedNonce,
	}).Info("Checked latest nonce delivered to ethereum incentivized channel")

	paraBasicNonceKey, err := types.CreateStorageKey(li.parachainConnection.GetMetadata(), "BasicOutboundModule", "Nonce", nil, nil)
	if err != nil {
		li.log.Error(err)
		return nil, err
	}
	var paraBasicNonce types.U64
	ok, err := li.parachainConnection.GetAPI().RPC.State.GetStorage(paraBasicNonceKey, &paraBasicNonce, paraHash)
	if err != nil {
		li.log.Error(err)
		return nil, err
	}
	if !ok {
		paraBasicNonce = 0
	}
	li.log.WithFields(logrus.Fields{
		"nonce": uint64(paraBasicNonce),
	}).Info("Checked latest nonce generated by parachain basic channel")

	paraIncentivizedNonceKey, err := types.CreateStorageKey(li.parachainConnection.GetMetadata(), "IncentivizedOutboundModule", "Nonce", nil, nil)
	if err != nil {
		li.log.Error(err)
		return nil, err
	}
	var paraIncentivizedNonce types.U64
	ok, err = li.parachainConnection.GetAPI().RPC.State.GetStorage(paraIncentivizedNonceKey, &paraIncentivizedNonce, paraHash)
	if err != nil {
		li.log.Error(err)
		return nil, err
	}
	if !ok {
		paraBasicNonce = 0
	}
	li.log.WithFields(logrus.Fields{
		"nonce": uint64(paraIncentivizedNonce),
	}).Info("Checked latest nonce generated by parachain incentivized channel")

	if ethBasicNonce == uint64(paraBasicNonce) && ethIncentivizedNonce == uint64(paraIncentivizedNonce) {
		return nil, err
	}

	paraBlocks, err := li.searchForLostCommitments(paraBlock, ethBasicNonce, ethIncentivizedNonce)
	if err != nil {
		return nil, err
	}

	li.log.Info("Stopped searching for lost commitments")

	li.log.WithFields(logrus.Fields{
		"blocks": paraBlocks,
	}).Info("Found these blocks and commitments")

	blocksWithProofs, err := li.ParablocksWithProofs(paraBlocks, relaychainBlock)
	if err != nil {
		li.log.Error(err)
		return nil, err
	}

	// Reverse blocks to be in ascending order
	for i, j := 0, len(blocksWithProofs)-1; i < j; i, j = i+1, j-1 {
		blocksWithProofs[i], blocksWithProofs[j] = blocksWithProofs[j], blocksWithProofs[i]
	}

	li.log.WithFields(logrus.Fields{
		"blocks": blocksWithProofs,
	}).Info("Packaging these blocks and proofs")

	messagePackets, err := li.createMessagePackets(blocksWithProofs)
	if err != nil {
		li.log.WithError(err).Error("Failed to create message packet")
		return nil, err
	}

	for _, messagePacket := range messagePackets {
		li.log.WithFields(logrus.Fields{
			"channelID":        messagePacket.channelID,
			"commitmentHash":   messagePacket.commitmentHash,
			"commitmentData":   messagePacket.commitmentData,
			"ourParaHeadProof": messagePacket.paraHeadProof,
			"mmrProof":         messagePacket.mmrProof,
		}).Info("Beefy Listener emitting new message packet")

		li.messages <- messagePacket
	}

	return messagePackets, nil
}

// Takes a slice of parachain blocks and augments them with their respective
// header, header proof and MMR proof
func (li *BeefyListener) ParablocksWithProofs(blocks []ParaBlockWithDigest, latestRelayChainBlockNumber uint64) (
	[]ParaBlockWithProofs, error) {
	relayChainBlockNumber := latestRelayChainBlockNumber
	var relayBlockHash types.Hash
	var allParaHeads []types.Header
	var ourParaHead types.Header
	var err error
	var blocksWithProof []ParaBlockWithProofs
	for _, block := range blocks {
		// Loop back over relay chain blocks to find the one that finalized the given parachain block
		for ourParaHead.Number != types.BlockNumber(block.BlockNumber) {
			li.log.WithField("relayChainBlockNumber", relayChainBlockNumber).Info("Getting hash for relay chain block")
			relayBlockHash, err = li.relaychainConn.GetAPI().RPC.Chain.GetBlockHash(uint64(relayChainBlockNumber))
			if err != nil {
				li.log.WithError(err).Error("Failed to get block hash")
				return nil, err
			}
			li.log.WithField("relayBlockHash", relayBlockHash.Hex()).Info("Got relay chain blockhash")
			allParaHeads, ourParaHead, err = li.relaychainConn.GetAllParaheadsWithOwn(relayBlockHash, OUR_PARACHAIN_ID)
			if err != nil {
				li.log.WithError(err).Error("Failed to get paraheads")
				return nil, err
			}
			relayChainBlockNumber--
		}

		// Note - relayChainBlockNumber will be one less than the block number discovered
		// due to the decrement at the end of the loop, but the mmr leaves are 0 indexed whereas
		// block numbers start from 1, so we actually do want to query for the leaf at
		// one less than the block number
		mmrProof, err := li.relaychainConn.GetMMRLeafForBlock(uint64(relayChainBlockNumber), relayBlockHash)
		if err != nil {
			li.log.WithError(err).Error("Failed to get mmr leaf")
			return nil, err
		}
		ourParaHeadProof, err := li.createParachainHeaderProof(allParaHeads, ourParaHead, mmrProof.Leaf.ParachainHeads)
		if err != nil {
			li.log.WithError(err).Error("Failed to create parachain header proof")
			return nil, err
		}

		blockWithProof := ParaBlockWithProofs{
			Block:            block,
			MMRProofResponse: mmrProof,
			Header:           ourParaHead,
			HeaderProof:      ourParaHeadProof,
		}
		blocksWithProof = append(blocksWithProof, blockWithProof)
	}
	return blocksWithProof, nil
}

func (li *BeefyListener) createParachainHeaderProof(allParaHeads []types.Header, ourParaHead types.Header, expectedRoot types.H256) (string, error) {
	//TODO: implement
	//TODO: check against expectedRoot
	return "", nil
}

func (li *BeefyListener) createMessagePackets(paraBlocks []ParaBlockWithProofs) ([]MessagePackage, error) {
	var messagePackages []MessagePackage

	for _, block := range paraBlocks {
		for _, item := range block.Block.DigestItemsWithData {
			li.log.WithFields(logrus.Fields{
				"block":          block.Block.BlockNumber,
				"channelID":      item.DigestItem.AsCommitment.ChannelID,
				"commitmentHash": item.DigestItem.AsCommitment.Hash.Hex(),
			}).Debug("Found commitment hash in header digest")
			commitmentHash := item.DigestItem.AsCommitment.Hash
			commitmentData := item.Data
			messagePackage := MessagePackage{
				item.DigestItem.AsCommitment.ChannelID,
				commitmentHash,
				commitmentData,
				block.Header,
				block.HeaderProof,
				block.MMRProofResponse,
			}
			messagePackages = append(messagePackages, messagePackage)
		}
	}

	return messagePackages, nil
}

// Searches for all lost commitments on each channel from the given parachain block number backwards
// until it finds the given basic and incentivized nonce
func (li *BeefyListener) searchForLostCommitments(
	lastParaBlockNumber uint64,
	basicNonceToFind uint64,
	incentivizedNonceToFind uint64) ([]ParaBlockWithDigest, error) {
	li.log.WithFields(logrus.Fields{
		"basicNonce":        basicNonceToFind,
		"incentivizedNonce": incentivizedNonceToFind,
		"latestblockNumber": lastParaBlockNumber,
	}).Debug("Searching backwards from latest block on parachain to find block with nonce")
	basicId := substrate.ChannelID{IsBasic: true}
	incentivizedId := substrate.ChannelID{IsIncentivized: true}

	currentBlockNumber := lastParaBlockNumber + 1
	basicNonceFound := false
	incentivizedNonceFound := false
	var blocks []ParaBlockWithDigest
	for (basicNonceFound == false || incentivizedNonceFound == false) && currentBlockNumber != 0 {
		currentBlockNumber--
		li.log.WithFields(logrus.Fields{
			"blockNumber": currentBlockNumber,
		}).Debug("Checking header...")

		blockHash, err := li.parachainConnection.GetAPI().RPC.Chain.GetBlockHash(currentBlockNumber)
		if err != nil {
			li.log.WithFields(logrus.Fields{
				"blockNumber": currentBlockNumber,
			}).WithError(err).Error("Failed to fetch blockhash")
			return nil, err
		}

		header, err := li.parachainConnection.GetAPI().RPC.Chain.GetHeader(blockHash)
		if err != nil {
			li.log.WithError(err).Error("Failed to fetch header")
			return nil, err
		}

		digestItems, err := li.getAuxiliaryDigestItems(header.Digest)
		if err != nil {
			return nil, err
		}

		var digestItemsWithData []DigestItemWithData

		for _, digestItem := range digestItems {
			if digestItem.IsCommitment {
				channelID := digestItem.AsCommitment.ChannelID
				if channelID == basicId && !basicNonceFound {
					isRelayed, messageData, err := li.checkBasicMessageNonces(&digestItem, basicNonceToFind)
					if err != nil {
						return nil, err
					}
					if isRelayed {
						basicNonceFound = true
					} else {
						item := DigestItemWithData{digestItem, messageData}
						digestItemsWithData = append(digestItemsWithData, item)
					}
				}
				if channelID == incentivizedId && !incentivizedNonceFound {
					isRelayed, messageData, err := li.checkIncentivizedMessageNonces(&digestItem, incentivizedNonceToFind)
					if err != nil {
						return nil, err
					}
					if isRelayed {
						incentivizedNonceFound = true
					} else {
						item := DigestItemWithData{digestItem, messageData}
						digestItemsWithData = append(digestItemsWithData, item)
					}
				}
			}
		}

		block := ParaBlockWithDigest{
			BlockNumber:         currentBlockNumber,
			DigestItemsWithData: digestItemsWithData,
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

func (li *BeefyListener) checkBasicMessageNonces(
	digestItem *chainTypes.AuxiliaryDigestItem,
	nonceToFind uint64,
) (bool, types.StorageDataRaw, error) {
	messages, data, err := li.getBasicMessages(*digestItem)
	if err != nil {
		return false, nil, err
	}

	for _, message := range messages {
		if message.Nonce <= nonceToFind {
			return true, data, nil
		}
	}
	return false, data, nil
}

func (li *BeefyListener) checkIncentivizedMessageNonces(
	digestItem *chainTypes.AuxiliaryDigestItem,
	nonceToFind uint64,
) (bool, types.StorageDataRaw, error) {
	messages, data, err := li.getIncentivizedMessages(*digestItem)
	if err != nil {
		return false, nil, err
	}

	for _, message := range messages {
		if message.Nonce <= nonceToFind {
			return true, data, nil
		}
	}
	return false, data, nil
}

func (li *BeefyListener) getBasicMessages(digestItem substrate.AuxiliaryDigestItem) (
	[]chainTypes.BasicOutboundChannelMessage, types.StorageDataRaw, error) {
	data, err := li.getDataForDigestItem(&digestItem)
	if err != nil {
		return nil, nil, err
	}

	var messages []chainTypes.BasicOutboundChannelMessage

	err = types.DecodeFromBytes(data, &messages)
	if err != nil {
		li.log.WithError(err).Error("Failed to decode commitment messages")
		return nil, nil, err
	}

	return messages, data, nil
}

func (li *BeefyListener) getIncentivizedMessages(digestItem substrate.AuxiliaryDigestItem) (
	[]chainTypes.IncentivizedOutboundChannelMessage, types.StorageDataRaw, error) {
	data, err := li.getDataForDigestItem(&digestItem)
	if err != nil {
		return nil, nil, err
	}

	var messages []chainTypes.IncentivizedOutboundChannelMessage

	err = types.DecodeFromBytes(data, &messages)
	if err != nil {
		li.log.WithError(err).Error("Failed to decode commitment messages")
		return nil, nil, err
	}

	return messages, data, nil
}

// Fetch the latest block of our parachain that has been finalized on the relay chain
func (li *BeefyListener) fetchLatestVerifiedBlocks(ctx context.Context) (uint64, uint64, types.Hash, error) {
	verifiedRelayBlockNumber, err := li.beefyLightClient.LatestBeefyBlock(&bind.CallOpts{
		Pending: false,
		Context: ctx,
	})
	if err != nil {
		li.log.WithError(err).Error("Failed to get latest verified relay chain block number from ethereum")
		return 0, 0, types.Hash{}, err
	}

	verifiedRelayBlockHash, err := li.relaychainConn.GetAPI().RPC.Chain.GetBlockHash(verifiedRelayBlockNumber)
	if err != nil {
		li.log.WithError(err).Error("Failed to get latest relay chain block hash from relay chain")
		return 0, 0, types.Hash{}, err
	}
	li.log.WithField("blockHash", verifiedRelayBlockHash.Hex()).
		Info("Got latest relaychain blockhash that has been verified")

	_, ourParaHead, err := li.relaychainConn.GetAllParaheadsWithOwn(verifiedRelayBlockHash, OUR_PARACHAIN_ID)
	if err != nil {
		li.log.WithError(err).Error("Failed to get parachain heads from relay chain")
		return 0, 0, types.Hash{}, err
	}

	verifiedParaBlockNumber := uint64(ourParaHead.Number)
	ourParaHeadHash, err := li.parachainConnection.Api().RPC.Chain.GetBlockHash(verifiedParaBlockNumber)
	if err != nil {
		li.log.WithError(err).Error("Failed to get parachain block hash")
		return 0, 0, types.Hash{}, err
	}

	return verifiedRelayBlockNumber, verifiedParaBlockNumber, ourParaHeadHash, nil
}

func (li *BeefyListener) getDataForDigestItem(digestItem *chainTypes.AuxiliaryDigestItem) (types.StorageDataRaw, error) {
	storageKey, err := parachain.MakeStorageKey(digestItem.AsCommitment.ChannelID, digestItem.AsCommitment.Hash)
	if err != nil {
		return nil, err
	}

	data, err := li.parachainConnection.GetAPI().RPC.Offchain.LocalStorageGet(offchain.Persistent, storageKey)
	if err != nil {
		li.log.WithError(err).Error("Failed to read commitment from offchain storage")
		return nil, err
	}

	if data != nil {
		li.log.WithFields(logrus.Fields{
			"commitmentSizeBytes": len(*data),
		}).Debug("Retrieved commitment from offchain storage")
	} else {
		li.log.WithError(err).Error("Commitment not found in offchain storage")
		return nil, err
	}

	return *data, nil
}

func (li *BeefyListener) getAuxiliaryDigestItems(digest types.Digest) ([]chainTypes.AuxiliaryDigestItem, error) {
	var auxDigestItems []chainTypes.AuxiliaryDigestItem
	for _, digestItem := range digest {
		if digestItem.IsOther {
			var auxDigestItem chainTypes.AuxiliaryDigestItem
			err := types.DecodeFromBytes(digestItem.AsOther, &auxDigestItem)
			if err != nil {
				return nil, err
			}
			auxDigestItems = append(auxDigestItems, auxDigestItem)
		}
	}
	return auxDigestItems, nil
}
