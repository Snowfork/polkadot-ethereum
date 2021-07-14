package store

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3" // required by gorm
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
	ValidatorAddresses         []byte
	SignedCommitment           []byte
	SerializedLatestMMRProof   []byte
	ContractID                 int64
	Status                     Status
	InitialVerificationTxHash  common.Hash
	CompleteOnBlock            uint64
	RandomSeed                 common.Hash
	CompleteVerificationTxHash common.Hash
}

func NewBeefyRelayInfo(
	validatorAddresses, signedCommitment []byte,
	contractID int64, status Status,
	initialVerificationTxHash common.Hash,
	completeOnBlock uint64, randomSeed,
	completeVerificationTxHash common.Hash,
) BeefyRelayInfo {
	return BeefyRelayInfo{
		ValidatorAddresses:         validatorAddresses,
		SignedCommitment:           signedCommitment,
		ContractID:                 contractID,
		Status:                     status,
		InitialVerificationTxHash:  initialVerificationTxHash,
		CompleteOnBlock:            completeOnBlock,
		RandomSeed:                 randomSeed,
		CompleteVerificationTxHash: completeVerificationTxHash,
	}
}

func (b *BeefyRelayInfo) ToBeefyJustification() (BeefyJustification, error) {
	var validatorAddresses []common.Address
	if err := json.Unmarshal(b.ValidatorAddresses, &validatorAddresses); err != nil {
		return BeefyJustification{}, err
	}

	var signedCommitment SignedCommitment
	if err := json.Unmarshal(b.SignedCommitment, &signedCommitment); err != nil {
		return BeefyJustification{}, err
	}

	return BeefyJustification{
		ValidatorAddresses: validatorAddresses,
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
	Path     string
	DB       *gorm.DB
	messages <-chan DatabaseCmd
	log      *logrus.Entry
}

func NewDatabase(messages <-chan DatabaseCmd, log *logrus.Entry) *Database {
	return &Database{
		Path:     "",
		DB:       nil,
		messages: messages,
		log:      log,
	}
}

func (d *Database) Initialize() error {
	tmpfile, err := ioutil.TempFile("", "beefy.*.db")
	if err != nil {
		return nil
	}
	tmpfile.Close()

	db, err := gorm.Open("sqlite3", tmpfile.Name())
	if err != nil {
		return err
	}

	var beefyRelayInfo BeefyRelayInfo
	if !db.HasTable(&beefyRelayInfo) {
		db.CreateTable(&beefyRelayInfo)
		db.Model(&beefyRelayInfo)
	}

	d.Path = tmpfile.Name()
	d.DB = db

	return nil
}

func (d *Database) Start(ctx context.Context, eg *errgroup.Group) error {
	eg.Go(func() error {
		var err1, err2 error

		err1 = d.writeLoop(ctx)

		d.log.Info("Shutting down DB")
		sqlDB := d.DB.DB()
		if sqlDB != nil {
			err2 = sqlDB.Close()
			if err2 != nil {
				d.log.WithError(err2).Error("Unable to close DB connection")
			}

			err2 = os.Remove(d.Path)
			if err2 != nil {
				d.log.WithError(err2).Error("Unable to delete DB file")
			}
		}

		return err1
	})

	return nil
}

func (d *Database) writeLoop(ctx context.Context) error {
	var mutex = &sync.Mutex{}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
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
