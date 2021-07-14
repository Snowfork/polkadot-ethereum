// Copyright 2021 Snowfork
// SPDX-License-Identifier: LGPL-3.0-only

package ethrelayer

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/sirupsen/logrus"

	"github.com/snowfork/go-substrate-rpc-client/v3/types"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/ethereum"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/parachain"
	"github.com/snowfork/polkadot-ethereum/relayer/crypto/sr25519"
)

type Worker struct {
	dataDir     string
	ethconfig  *ethereum.Config
	ethconn    *ethereum.Connection
	paraconfig *parachain.Config
	paraconn   *parachain.Connection
	log        *logrus.Entry
}

const Name = "eth-relayer"

func NewWorker(
	dataDir   string,
	ethconfig *ethereum.Config,
	paraconfig *parachain.Config,
	log *logrus.Entry,
) *Worker {
	return &Worker{
		dataDir:    dataDir,
		ethconfig:  ethconfig,
		paraconfig: paraconfig,
		log:        log,
	}
}

func (w *Worker) Name() string {
	return Name
}

func (w *Worker) Start(ctx context.Context, eg *errgroup.Group) error {
	err := w.connect(ctx)
	if err != nil {
		return err
	}

	// Clean up after ourselves
	eg.Go(func() error {
		<-ctx.Done()
		w.disconnect()
		return nil
	})

	// channel for payloads from ethereum
	payloads := make(chan ParachainPayload, 1)

	listener := NewEthereumListener(
		w.dataDir,
		w.ethconfig,
		w.ethconn,
		payloads,
		w.log,
	)
	writer := NewParachainWriter(
		w.paraconn,
		payloads,
		w.log,
	)

	finalizedBlockNumber, err := w.queryFinalizedBlockNumber()
	if err != nil {
		return err
	}
	w.log.WithField("blockNumber", finalizedBlockNumber).Debug("Retrieved finalized block number from parachain")

	err = listener.Start(ctx, eg, finalizedBlockNumber+1, uint64(w.ethconfig.DescendantsUntilFinal))
	if err != nil {
		return err
	}

	err = writer.Start(ctx, eg)
	if err != nil {
		return err
	}

	return nil
}

func (w *Worker) queryFinalizedBlockNumber() (uint64, error) {
	storageKey, err := types.CreateStorageKey(w.paraconn.Metadata(), "VerifierLightclient", "FinalizedBlock", nil, nil)
	if err != nil {
		return 0, err
	}

	var finalizedHeader ethereum.HeaderID
	_, err = w.paraconn.Api().RPC.State.GetStorageLatest(storageKey, &finalizedHeader)
	if err != nil {
		return 0, err
	}

	return uint64(finalizedHeader.Number), nil
}

func (w *Worker) connect(ctx context.Context) error {
	kpForPara, err := sr25519.NewKeypairFromSeed(w.paraconfig.PrivateKey, 42)
	if err != nil {
		return err
	}

	w.ethconn = ethereum.NewConnection(w.ethconfig.Endpoint, nil, w.log)
	w.paraconn = parachain.NewConnection(w.paraconfig.Endpoint, kpForPara.AsKeyringPair(), w.log)

	err = w.ethconn.Connect(ctx)
	if err != nil {
		return err
	}

	err = w.paraconn.Connect(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (w *Worker) disconnect() {
	if w.ethconn != nil {
		w.ethconn.Close()
		w.ethconn = nil
	}

	if w.paraconn != nil {
		w.paraconn.Close()
		w.paraconn = nil
	}
}
