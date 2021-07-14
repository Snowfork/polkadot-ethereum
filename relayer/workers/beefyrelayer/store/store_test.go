package store_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/sirupsen/logrus"
	"github.com/snowfork/go-substrate-rpc-client/v3/types"
	"github.com/snowfork/polkadot-ethereum/relayer/workers/beefyrelayer/store"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sync/errgroup"
)

type StoreTestSuite struct {
	suite.Suite

	database *store.Database
	messages chan store.DatabaseCmd
	ctx      context.Context
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (suite *StoreTestSuite) SetupTest() {

	messages := make(chan store.DatabaseCmd, 1)
	logger := logrus.WithField("database", "Beefy")
	database := store.NewDatabase(messages, logger)

	err := database.Initialize()
	if err != nil {
		suite.Fail(err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	eg, ctx := errgroup.WithContext(ctx)
	// Ensure clean termination upon SIGINT, SIGTERM
	eg.Go(func() error {
		notify := make(chan os.Signal, 1)
		signal.Notify(notify, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig := <-notify:
			logrus.WithField("signal", sig.String()).Info("Received signal")
			cancel()
		}
		return nil
	})

	suite.messages = messages
	suite.database = database
	suite.ctx = ctx

	// Start database update loop
	err = database.Start(ctx, eg)
	if err != nil {
		panic(err)
	}
}

func (suite *StoreTestSuite) TestGetItemsByStatus() {
	item := loadSampleBeefyRelayInfo()

	// Pass create command to write loop
	createCmd := store.NewDatabaseCmd(&item, store.Create, nil)
	suite.messages <- createCmd

	time.Sleep(2 * time.Second)

	items := suite.database.GetItemsByStatus(store.CommitmentWitnessed)
	beefyItem := items[0]
	suite.Equal(beefyItem.Status, store.CommitmentWitnessed)
}

func (suite *StoreTestSuite) TestGetItemByID() {
	id := int64(55)
	item := loadSampleBeefyRelayInfo()
	item.ContractID = id

	// Pass create command to write loop
	createCmd := store.NewDatabaseCmd(&item, store.Create, nil)
	suite.messages <- createCmd

	time.Sleep(2 * time.Second)

	foundItem := suite.database.GetItemByID(id)
	suite.Equal(item.ID, foundItem.ID)
}

func (suite *StoreTestSuite) TestGetItemByInitialVerificationTxHash() {
	hash := common.BytesToHash([]byte("0x25451A4de12dcCc2D166922fA938E900fCc4ED24"))
	item := loadSampleBeefyRelayInfo()
	item.InitialVerificationTxHash = hash

	// Pass create command to write loop
	createCmd := store.NewDatabaseCmd(&item, store.Create, nil)
	suite.messages <- createCmd

	time.Sleep(2 * time.Second)

	foundItem := suite.database.GetItemByInitialVerificationTxHash(hash)
	suite.Equal(item.InitialVerificationTxHash, foundItem.InitialVerificationTxHash)
}

func (suite *StoreTestSuite) TestGetItemByCompleteVerificationTxHash() {
	hash := common.BytesToHash([]byte("0x25451A4de12dcCc2D166922fA938E900fCc4ED24"))
	item := loadSampleBeefyRelayInfo()
	item.CompleteVerificationTxHash = hash

	// Pass create command to write loop
	createCmd := store.NewDatabaseCmd(&item, store.Create, nil)
	suite.messages <- createCmd

	time.Sleep(2 * time.Second)

	foundItem := suite.database.GetItemByCompleteVerificationTxHash(hash)
	suite.Equal(item.CompleteVerificationTxHash, foundItem.CompleteVerificationTxHash)
}

func (suite *StoreTestSuite) TestUpdateItem() {
	item := loadSampleBeefyRelayInfo()

	// Pass create command to write loop
	createCmd := store.NewDatabaseCmd(&item, store.Create, nil)
	suite.messages <- createCmd

	time.Sleep(2 * time.Second)

	items := suite.database.GetItemsByStatus(store.CommitmentWitnessed)
	beefyItem := items[0]

	// Pass update instruction to write loop
	hash := common.BytesToHash([]byte("0x25451A4de12dcCc2D166922fA938E900fCc4ED24"))
	instructions := map[string]interface{}{
		"status":                    store.InitialVerificationTxSent,
		"InitialVerificationTxHash": hash,
	}
	updateCmd := store.NewDatabaseCmd(beefyItem, store.Update, instructions)
	suite.messages <- updateCmd

	time.Sleep(2 * time.Second)

	newItem := suite.database.GetItemByInitialVerificationTxHash(hash)
	suite.Equal(newItem.Status, store.InitialVerificationTxSent)
	suite.Equal(newItem.InitialVerificationTxHash, hash)
}

func (suite *StoreTestSuite) TestDeleteItem() {
	id := int64(88)
	item := loadSampleBeefyRelayInfo()
	item.ContractID = id

	// Pass create command to write loop
	createCmd := store.NewDatabaseCmd(&item, store.Create, nil)
	suite.messages <- createCmd

	time.Sleep(2 * time.Second)

	foundItem := suite.database.GetItemByID(id)
	suite.Equal(item.ID, foundItem.ID)
	suite.Equal(item.CompleteOnBlock, foundItem.CompleteOnBlock)

	// Pass delete command to write loop
	deleteCmd := store.NewDatabaseCmd(&item, store.Delete, nil)
	suite.messages <- deleteCmd

	time.Sleep(2 * time.Second)

	deletedItem := suite.database.GetItemByID(id)
	suite.Equal(store.Status(0), deletedItem.Status)
	suite.Equal(uint(0), deletedItem.ID)
	suite.Equal(uint64(0), deletedItem.CompleteOnBlock)
}

func loadSampleBeefyRelayInfo() store.BeefyRelayInfo {
	// Sample BEEFY commitment: validator addresses
	beefyValidatorAddresses := []common.Address{
		common.HexToAddress("0xE04CC55ebEE1cBCE552f250e85c57B70B2E2625b"),
		common.HexToAddress("0x25451A4de12dcCc2D166922fA938E900fCc4ED24"),
	}

	payload := "9db1a857a25a997190c920b7f333eef069804639546acc411e7937a13c592fd2"
	payloadBytes, err := hex.DecodeString(payload)
	if err != nil {
		panic(err)
	}

	sig1 := "d0834df8b658963611deecf57b845f906f517c1a9b9467f31e8dc292d1a131d22ccb2201dc6e49043fce104418442f9d20acaa3c5d86d2ce20285de0e64b057a01"
	sig1Bytes, err := hex.DecodeString(sig1)
	if err != nil {
		panic(err)
	}
	var sig1Input [65]byte
	copy(sig1Input[:], sig1Bytes)
	beefySig1 := store.BeefySignature(sig1Input)

	sig2 := "daf339e4e248cdc46b4b84640ffc3987ab843ab336b9e39fcf7cc9bb65841b2a0b224d5116c8b0e7b7bb3a99df92d53d4ffe0f7857a753ada3d28be130585ab801"
	sig2Bytes, err := hex.DecodeString(sig2)
	if err != nil {
		panic(err)
	}
	var sig2Input [65]byte
	copy(sig2Input[:], sig2Bytes)
	beefySig2 := store.BeefySignature(sig2Input)

	signedCommitment := store.SignedCommitment{
		Commitment: store.Commitment{
			Payload:        types.NewH256(payloadBytes),
			BlockNumber:    types.NewU32(930),
			ValidatorSetID: types.NewU64(0),
		},
		Signatures: []store.OptionBeefySignature{
			store.NewOptionBeefySignature(beefySig1),
			store.NewOptionBeefySignature(beefySig2),
		},
	}

	valAddrBytes, err := json.Marshal(beefyValidatorAddresses)
	if err != nil {
		panic(err)
	}

	signedCommitmentBytes, err := json.Marshal(signedCommitment)
	if err != nil {
		panic(err)
	}

	return store.BeefyRelayInfo{
		ValidatorAddresses:        valAddrBytes,
		SignedCommitment:          signedCommitmentBytes,
		ContractID:                int64(1),
		Status:                    store.CommitmentWitnessed,
		InitialVerificationTxHash: common.Hash{},
		CompleteOnBlock:           22,
	}
}
