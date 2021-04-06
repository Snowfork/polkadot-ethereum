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

type BeefyItem struct {
	gorm.Model
	ValidatorAddresses         []byte
	SignedCommitment           []byte
	Status                     Status
	InitialVerificationTxHash  common.Hash
	CompleteOnBlock            uint64
	RandomSeed                 common.Hash
	CompleteVerificationTxHash common.Hash
}

func NewBeefyItem(validatorAddresses, signedCommitment []byte, status Status,
	initialVerificationTxHash common.Hash, completeOnBlock uint64, randomSeed,
	completeVerificationTxHash common.Hash) BeefyItem {
	return BeefyItem{
		ValidatorAddresses:         validatorAddresses,
		SignedCommitment:           signedCommitment,
		Status:                     status,
		InitialVerificationTxHash:  initialVerificationTxHash,
		CompleteOnBlock:            completeOnBlock,
		RandomSeed:                 randomSeed,
		CompleteVerificationTxHash: completeVerificationTxHash,
	}
}

func (b *BeefyItem) ToBeefy() (Beefy, error) {
	var validatorAddresses []common.Address
	if err := json.Unmarshal(b.ValidatorAddresses, &validatorAddresses); err != nil {
		return Beefy{}, err
	}

	var signedCommitment SignedCommitment
	if err := json.Unmarshal(b.SignedCommitment, &signedCommitment); err != nil {
		return Beefy{}, err
	}

	return Beefy{
		ValidatorAddresses: validatorAddresses,
		SignedCommitment:   signedCommitment,
	}, nil
}

func (BeefyItem) TableName() string {
	return "beefy_item"
}

type CmdType int

const (
	Create CmdType = iota // 0
	Update CmdType = iota // 1
)

type DatabaseCmd struct {
	Item         *BeefyItem
	Type         CmdType
	Instructions map[string]interface{}
}

func NewDatabaseCmd(item *BeefyItem, cmdType CmdType, instructions map[string]interface{}) DatabaseCmd {
	return DatabaseCmd{
		Item:         item,
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
	var beefyItem BeefyItem
	if !db.HasTable(&beefyItem) {
		db.CreateTable(&beefyItem)
		db.Model(&beefyItem)
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
				}

				if err := tx.Create(&cmd.Item).Error; err != nil {
					tx.Rollback()
					d.log.Error(err)
				}

				if err := tx.Commit().Error; err != nil {
					d.log.Error(err)
				}
			case Update:
				d.log.Info("Updating item in database...")
				d.DB.Model(&cmd.Item).Updates(cmd.Instructions)
			}
			mutex.Unlock()
		}
	}
}

func (d *Database) GetItemsByStatus(status Status) []*BeefyItem {
	items := make([]*BeefyItem, 0)
	d.DB.Where("status = ?", status).Find(&items)
	return items
}

func (d *Database) GetItemByInitialVerificationTxHash(txHash common.Hash) *BeefyItem {
	var item BeefyItem
	d.DB.Take(&item, "initial_verification_tx_hash = ?", txHash)
	return &item
}
