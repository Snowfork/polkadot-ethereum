package beefyrelayer

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"golang.org/x/sync/errgroup"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"

	"github.com/snowfork/polkadot-ethereum/relayer/chain/ethereum"
	"github.com/snowfork/polkadot-ethereum/relayer/contracts/beefylightclient"
	"github.com/snowfork/polkadot-ethereum/relayer/workers/beefyrelayer/store"
)

type BeefyEthereumWriter struct {
	ethereumConfig   *ethereum.Config
	ethereumConn     *ethereum.Connection
	beefyDB          *store.Database
	beefyLightClient *beefylightclient.Contract
	databaseMessages chan<- store.DatabaseCmd
	beefyMessages    <-chan store.BeefyRelayInfo
	log              *logrus.Entry
}

func NewBeefyEthereumWriter(ethereumConfig *ethereum.Config, ethereumConn *ethereum.Connection, beefyDB *store.Database,
	databaseMessages chan<- store.DatabaseCmd, beefyMessages <-chan store.BeefyRelayInfo,
	log *logrus.Entry) *BeefyEthereumWriter {
	return &BeefyEthereumWriter{
		ethereumConfig:   ethereumConfig,
		ethereumConn:     ethereumConn,
		beefyDB:          beefyDB,
		databaseMessages: databaseMessages,
		beefyMessages:    beefyMessages,
		log:              log,
	}
}

func (wr *BeefyEthereumWriter) Start(ctx context.Context, eg *errgroup.Group) error {

	beefyLightClientContract, err := beefylightclient.NewContract(common.HexToAddress(wr.ethereumConfig.BeefyLightClient), wr.ethereumConn.GetClient())
	if err != nil {
		return err
	}
	wr.beefyLightClient = beefyLightClientContract

	eg.Go(func() error {
		return wr.writeMessagesLoop(ctx)
	})

	return nil
}

func (wr *BeefyEthereumWriter) onDone(ctx context.Context) error {
	wr.log.Info("Shutting down writer...")
	// Avoid deadlock if a listener is still trying to send to a channel
	for range wr.beefyMessages {
		wr.log.Debug("Discarded BEEFY message")
	}
	return ctx.Err()
}

func (wr *BeefyEthereumWriter) writeMessagesLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return wr.onDone(ctx)
		case msg := <-wr.beefyMessages:
			switch msg.Status {
			case store.CommitmentWitnessed:
				err := wr.WriteNewSignatureCommitment(ctx, msg, 0) // TODO: pick val addr
				if err != nil {
					wr.log.WithError(err).Error("Error submitting message to ethereum")
				}
			case store.ReadyToComplete:
				err := wr.WriteCompleteSignatureCommitment(ctx, msg)
				if err != nil {
					wr.log.WithError(err).Error("Error submitting message to ethereum")
				}
			}
		}
	}
}

func (wr *BeefyEthereumWriter) signerFn(_ common.Address, tx *types.Transaction) (*types.Transaction, error) {
	signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, wr.ethereumConn.GetKP().PrivateKey())
	if err != nil {
		return nil, err
	}
	return signedTx, nil
}

func (wr *BeefyEthereumWriter) WriteNewSignatureCommitment(ctx context.Context, info store.BeefyRelayInfo, valIndex int) error {
	beefyJustification, err := info.ToBeefyJustification()
	if err != nil {
		return fmt.Errorf("Error converting BeefyRelayInfo to BeefyJustification: %s", err.Error())
	}

	contract := wr.beefyLightClient
	if contract == nil {
		return fmt.Errorf("Unknown contract")
	}

	signedValidators := []*big.Int{}
	for i := range beefyJustification.SignedCommitment.Signatures {
		// TODO: skip over empty/missing signatures
		// if signature.Option.IsSome() {
		signedValidators = append(signedValidators, big.NewInt(int64(i)))
		// }
	}
	numberOfValidators := big.NewInt(int64(len(beefyJustification.SignedCommitment.Signatures)))
	initialBitfield, err := contract.CreateInitialBitfield(
		&bind.CallOpts{Pending: true}, signedValidators, numberOfValidators,
	)
	if err != nil {
		wr.log.WithError(err).Error("Failed to create initial validator bitfield")
		return err
	}

	msg, err := beefyJustification.BuildNewSignatureCommitmentMessage(valIndex, initialBitfield)
	if err != nil {
		return err
	}

	options := bind.TransactOpts{
		From:     wr.ethereumConn.GetKP().CommonAddress(),
		Signer:   wr.signerFn,
		Context:  ctx,
		GasLimit: 5000000,
	}

	tx, err := contract.NewSignatureCommitment(&options, msg.CommitmentHash,
		msg.ValidatorClaimsBitfield, msg.ValidatorSignatureCommitment,
		msg.ValidatorPosition, msg.ValidatorPublicKey, msg.ValidatorPublicKeyMerkleProof)
	if err != nil {
		wr.log.WithError(err).Error("Failed to submit transaction")
		return err
	}

	wr.log.WithFields(logrus.Fields{
		"txHash": tx.Hash().Hex(),
	}).Info("New Signature Commitment transaction submitted")

	wr.log.Info("1: Creating item in Database with status 'InitialVerificationTxSent'")
	info.Status = store.InitialVerificationTxSent
	info.InitialVerificationTxHash = tx.Hash()
	cmd := store.NewDatabaseCmd(&info, store.Create, nil)
	wr.databaseMessages <- cmd

	return nil
}

// WriteCompleteSignatureCommitment sends a CompleteSignatureCommitment tx to the BeefyLightClient contract
func (wr *BeefyEthereumWriter) WriteCompleteSignatureCommitment(ctx context.Context, info store.BeefyRelayInfo) error {
	beefyJustification, err := info.ToBeefyJustification()
	if err != nil {
		return fmt.Errorf("Error converting BeefyRelayInfo to BeefyJustification: %s", err.Error())
	}

	contract := wr.beefyLightClient
	if contract == nil {
		return fmt.Errorf("Unknown contract")
	}

	randomBitfield, err := contract.CreateRandomBitfield(
		&bind.CallOpts{Pending: true},
		big.NewInt(int64(info.ContractID)),
	)
	if err != nil {
		wr.log.WithError(err).Error("Failed to get random validator bitfield")
		return err
	}

	bitfield := ""
	for _, bitfieldInt := range randomBitfield {
		bits := strconv.FormatInt(bitfieldInt.Int64(), 2)
		bitfield += bits
	}

	msg, err := beefyJustification.BuildCompleteSignatureCommitmentMessage(info, bitfield)
	if err != nil {
		return err
	}

	options := bind.TransactOpts{
		From:     wr.ethereumConn.GetKP().CommonAddress(),
		Signer:   wr.signerFn,
		Context:  ctx,
		GasLimit: 2000000,
	}

	validatorProof := beefylightclient.BeefyLightClientValidatorProof{
		Signatures:            msg.Signatures,
		Positions:             msg.ValidatorPositions,
		PublicKeys:            msg.ValidatorPublicKeys,
		PublicKeyMerkleProofs: msg.ValidatorPublicKeyMerkleProofs,
	}

	tx, err := contract.CompleteSignatureCommitment(&options,
		msg.ID,
		msg.Commitment,
		validatorProof,
		msg.LatestMMRLeaf,
		msg.MMRProofItems)

	if err != nil {
		wr.log.WithError(err).Error("Failed to submit transaction")
		return err
	}

	wr.log.WithFields(logrus.Fields{
		"txHash": tx.Hash().Hex(),
	}).Info("Complete Signature Commitment transaction submitted")

	// Update item's status in database
	wr.log.Info("5: Updating item status from 'ReadyToComplete' to 'CompleteVerificationTxSent'")
	instructions := map[string]interface{}{
		"status":                        store.CompleteVerificationTxSent,
		"complete_verification_tx_hash": tx.Hash(),
	}
	updateCmd := store.NewDatabaseCmd(&info, store.Update, instructions)
	wr.databaseMessages <- updateCmd

	wr.LogBeefyFixtureDataAll(msg, info)
	msgString, err := wr.GetFailingMessage(*wr.ethereumConn.GetClient(), tx.Hash())
	wr.log.WithFields(logrus.Fields{
		"err": err,
		"msg": msgString,
	}).Info("Failing message")

	return nil
}
