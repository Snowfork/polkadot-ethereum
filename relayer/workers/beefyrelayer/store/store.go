package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type Status int

const (
	CommitmentWitnessed            Status = iota // 0
	InitialVerificationTxSent      Status = iota // 1
	InitialVerificationTxConfirmed Status = iota // 2
	ReadyToComplete                Status = iota // 3
	CompleteVerificationTxSent     Status = iota // 4
)

type BeefyRelayInfo struct {
	gorm.Model
	ValidatorAddresses         []string
	SignedCommitment           []byte
	SerializedLatestMMRProof   []byte
	ContractID                 int64
	Status                     Status
	InitialVerificationTxHash  common.Hash
	CompleteOnBlock            uint64
	RandomSeed                 common.Hash
	CompleteVerificationTxHash common.Hash
}

func NewBeefyRelayInfo(validatorAddresses []string, signedCommitment []byte, contractId int64, status Status,
	initialVerificationTxHash common.Hash, completeOnBlock uint64, randomSeed,
	completeVerificationTxHash common.Hash) BeefyRelayInfo {
	return BeefyRelayInfo{
		ValidatorAddresses:         validatorAddresses,
		SignedCommitment:           signedCommitment,
		ContractID:                 contractId,
		Status:                     status,
		InitialVerificationTxHash:  initialVerificationTxHash,
		CompleteOnBlock:            completeOnBlock,
		RandomSeed:                 randomSeed,
		CompleteVerificationTxHash: completeVerificationTxHash,
	}
}

func (b *BeefyRelayInfo) ToBeefyJustification() (BeefyJustification, error) {
	var signedCommitment SignedCommitment
	if err := json.Unmarshal(b.SignedCommitment, &signedCommitment); err != nil {
		return BeefyJustification{}, err
	}

	return BeefyJustification{
		ValidatorAddresses: b.ValidatorAddresses,
		SignedCommitment:   signedCommitment,
	}, nil
}

func (BeefyRelayInfo) TableName() string {
	return "beefy_relay_info"
}

type CmdType int

const (
	Create CmdType = iota // 0
	Update CmdType = iota // 1
	Delete CmdType = iota // 2
)

type DatabaseCmd struct {
	Info         *BeefyRelayInfo
	Type         CmdType
	Instructions map[string]interface{}
}

func NewDatabaseCmd(info *BeefyRelayInfo, cmdType CmdType, instructions map[string]interface{}) DatabaseCmd {
	return DatabaseCmd{
		Info:         info,
		Type:         cmdType,
		Instructions: instructions,
	}
}

type Database struct {
	DB       *gorm.DB
	messages <-chan DatabaseCmd
	log      *logrus.Entry
}

func NewDatabase(db *gorm.DB, messages <-chan DatabaseCmd, log *logrus.Entry) *Database {
	return &Database{
		DB:       db,
		messages: messages,
		log:      log,
	}
}

func PrepareDatabase(config *Config) (*gorm.DB, error) {
	if len(config.DBPath) == 0 {
		return nil, fmt.Errorf("invalid database path: %s", config.DBPath)
	}
	tmpDBFile, err := ioutil.TempFile("", config.DBPath)
	if err != nil {
		return nil, err
	}

	db, err := gorm.Open(config.Dialect, tmpDBFile.Name())
	if err != nil {
		return nil, err
	}

	InitTables(db)

	return db, nil
}

func InitTables(db *gorm.DB) {
	var beefyRelayInfo BeefyRelayInfo
	if !db.HasTable(&beefyRelayInfo) {
		db.CreateTable(&beefyRelayInfo)
		db.Model(&beefyRelayInfo)
	}
}

func (d *Database) onDone(ctx context.Context) error {
	d.log.Info("Shutting down database...")
	return ctx.Err()
}

func (d *Database) Start(ctx context.Context, eg *errgroup.Group) error {
	eg.Go(func() error {
		return d.writeLoop(ctx)
	})

	return nil
}

// Stop is used to handle shut down logic
func (d *Database) Stop() {
	// Should automatically close. The database.close() method was removed in gorm 1.20.
}

func (d *Database) writeLoop(ctx context.Context) error {
	var mutex = &sync.Mutex{}

	for {
		select {
		case <-ctx.Done():
			return d.onDone(ctx)
		case cmd := <-d.messages:
			mutex.Lock()
			switch cmd.Type {
			case Create:
				d.log.Info("Creating item in database...")
				tx := d.DB.Begin()
				if err := tx.Error; err != nil {
					d.log.Error(err)
					return err
				}

				if err := tx.Create(&cmd.Info).Error; err != nil {
					tx.Rollback()
					d.log.Error(err)
					return err
				}

				if err := tx.Commit().Error; err != nil {
					d.log.Error(err)
					return err
				}
			case Update:
				d.log.Info("Updating item in database...")
				d.DB.Model(&cmd.Info).Updates(cmd.Instructions)
			case Delete:
				d.log.Info("Deleting item from database...")
				d.DB.Delete(&cmd.Info, cmd.Info.ID)
			}
			mutex.Unlock()
		}
	}
}

func (d *Database) GetItemsByStatus(status Status) []*BeefyRelayInfo {
	items := make([]*BeefyRelayInfo, 0)
	d.DB.Where("status = ?", status).Find(&items)
	return items
}

func (d *Database) GetItemByID(id int64) *BeefyRelayInfo {
	var item BeefyRelayInfo
	d.DB.Take(&item, "contract_id = ?", id)
	return &item
}

func (d *Database) GetItemByInitialVerificationTxHash(txHash common.Hash) *BeefyRelayInfo {
	var item BeefyRelayInfo
	d.DB.Take(&item, "initial_verification_tx_hash = ?", txHash)
	return &item
}

func (d *Database) GetItemByCompleteVerificationTxHash(txHash common.Hash) *BeefyRelayInfo {
	var item BeefyRelayInfo
	d.DB.Take(&item, "complete_verification_tx_hash = ?", txHash)
	return &item
}
