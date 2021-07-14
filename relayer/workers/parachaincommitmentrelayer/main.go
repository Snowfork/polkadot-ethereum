package parachaincommitmentrelayer

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/sirupsen/logrus"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/ethereum"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/parachain"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/relaychain"
	"github.com/snowfork/polkadot-ethereum/relayer/crypto/secp256k1"
)

type Worker struct {
	parachainConfig       *parachain.Config
	relaychainConfig      *relaychain.Config
	ethereumConfig        *ethereum.Config
	parachainConn         *parachain.Connection
	relaychainConn        *relaychain.Connection
	ethereumConn          *ethereum.Connection
	ethereumChannelWriter *EthereumChannelWriter
	beefyListener         *BeefyListener
	log                   *logrus.Entry
}

const Name = "parachain-commitment-relayer"

func NewWorker(parachainConfig *parachain.Config,
	relaychainConfig *relaychain.Config, ethereumConfig *ethereum.Config, log *logrus.Entry) (*Worker, error) {

	log.Info("Creating worker")

	ethereumKp, err := secp256k1.NewKeypairFromString(ethereumConfig.MessagePrivateKey)
	if err != nil {
		return nil, err
	}

	parachainConn := parachain.NewConnection(parachainConfig.Endpoint, nil, log)
	relaychainConn := relaychain.NewConnection(relaychainConfig.Endpoint, log)
	ethereumConn := ethereum.NewConnection(ethereumConfig.Endpoint, ethereumKp, log)

	// channel for messages from beefy listener to ethereum writer
	var messagePackages = make(chan MessagePackage, 1)

	ethereumChannelWriter, err := NewEthereumChannelWriter(
		ethereumConfig,
		ethereumConn,
		messagePackages,
		log,
	)
	if err != nil {
		return nil, err
	}

	beefyListener := NewBeefyListener(
		ethereumConfig,
		ethereumConn,
		relaychainConn,
		parachainConn,
		messagePackages,
		log,
	)

	return &Worker{
		parachainConfig:       parachainConfig,
		relaychainConfig:      relaychainConfig,
		ethereumConfig:        ethereumConfig,
		parachainConn:         parachainConn,
		relaychainConn:        relaychainConn,
		ethereumConn:          ethereumConn,
		ethereumChannelWriter: ethereumChannelWriter,
		beefyListener:         beefyListener,
		log:                   log,
	}, nil
}

func (worker *Worker) Start(ctx context.Context, eg *errgroup.Group) error {
	worker.log.Info("Starting worker")

	if worker.beefyListener == nil || worker.ethereumChannelWriter == nil {
		return fmt.Errorf("Sender and/or receiver need to be set before starting chain")
	}

	err := worker.parachainConn.Connect(ctx)
	if err != nil {
		return err
	}

	err = worker.ethereumConn.Connect(ctx)
	if err != nil {
		return err
	}

	err = worker.relaychainConn.Connect(ctx)
	if err != nil {
		return err
	}

	eg.Go(func() error {
		if worker.ethereumChannelWriter != nil {
			worker.log.Info("Starting Writer")
			err = worker.ethereumChannelWriter.Start(ctx, eg)
			if err != nil {
				return err
			}
		}
		return nil
	})

	eg.Go(func() error {
		if worker.beefyListener != nil {
			worker.log.Info("Starting Beefy Listener")
			err = worker.beefyListener.Start(ctx, eg)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return nil
}

func (worker *Worker) Stop() {
	if worker.parachainConn != nil {
		worker.parachainConn.Close()
	}
	if worker.relaychainConn != nil {
		worker.relaychainConn.Close()
	}
	if worker.ethereumConn != nil {
		worker.ethereumConn.Close()
	}
}

func (worker *Worker) Name() string {
	return Name
}
