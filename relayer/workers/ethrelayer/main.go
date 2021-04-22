// Copyright 2021 Snowfork
// SPDX-License-Identifier: LGPL-3.0-only

package ethrelayer

import (
	"context"

	"golang.org/x/sync/errgroup"

	"github.com/sirupsen/logrus"

	"github.com/snowfork/go-substrate-rpc-client/v2/types"
	"github.com/snowfork/polkadot-ethereum/relayer/chain"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/ethereum"
	"github.com/snowfork/polkadot-ethereum/relayer/chain/parachain"
	"github.com/snowfork/polkadot-ethereum/relayer/crypto/secp256k1"
	"github.com/snowfork/polkadot-ethereum/relayer/crypto/sr25519"
)

type Worker struct {
	ethconfig  *ethereum.Config
	ethconn    *ethereum.Connection
	paraconfig *parachain.Config
	paraconn   *parachain.Connection
	log        *logrus.Entry
}

const Name = "eth-relayer"

func NewWorker(ethconfig *ethereum.Config, paraconfig *parachain.Config, log *logrus.Entry) *Worker {
	return &Worker{
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

	// channel for messages from ethereum
	ethMessages := make(chan []chain.Message, 1)
	// channel for headers from ethereum (it's a blocking channel so that we
	// can guarantee that a header is forwarded before we send dependent messages)
	ethHeaders := make(chan chain.Header)

	listener := NewEthereumListener(
		w.ethconfig,
		w.ethconn,
		ethMessages,
		ethHeaders,
		w.log,
	)
	writer := NewParachainWriter(
		w.paraconn,
		ethMessages,
		ethHeaders,
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
	kpForEth, err := secp256k1.NewKeypairFromString(w.ethconfig.PrivateKey)
	if err != nil {
		return err
	}

	kpForPara, err := sr25519.NewKeypairFromSeed(w.paraconfig.PrivateKey, "")
	if err != nil {
		return err
	}

	w.ethconn = ethereum.NewConnection(w.ethconfig.Endpoint, kpForEth, w.log)
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

	if w.ethconn != nil {
		w.paraconn.Close()
		w.paraconn = nil
	}
}