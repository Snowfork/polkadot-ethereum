// Copyright 2020 Snowfork
// SPDX-License-Identifier: LGPL-3.0-only

package substrate

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/snowfork/go-substrate-rpc-client/v2/types"
	"golang.org/x/sync/errgroup"
)

const MaxWatchedExtrinsics = 500

type extrinsicPool struct {
	sync.Mutex
	conn     *Connection
	eg       *errgroup.Group
	log      *logrus.Entry
	maxNonce uint32
	watched  uint
}

func newExtrinsicPool(eg *errgroup.Group, conn *Connection, log *logrus.Entry) *extrinsicPool {
	ep := extrinsicPool{
		conn:    conn,
		eg:      eg,
		log:     log,
		watched: 0,
	}
	return &ep
}

func (ep *extrinsicPool) WaitForSubmitAndWatch(ctx context.Context, nonce uint32, ext *types.Extrinsic) {
	defer ep.Unlock()

	for {
		ep.Lock()
		if ep.hasCapacity() {
			ep.eg.Go(func() error {
				return ep.submitAndWatchLoop(ctx, nonce, ext)
			})

			ep.watched++
			return
		}
		ep.Unlock()

		time.Sleep(100 * time.Millisecond)
	}
}

func (ep *extrinsicPool) hasCapacity() bool {
	return ep.watched < MaxWatchedExtrinsics
}

func (ep *extrinsicPool) submitAndWatchLoop(ctx context.Context, nonce uint32, ext *types.Extrinsic) error {
	sub, err := ep.conn.api.RPC.Author.SubmitAndWatchExtrinsic(*ext)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Context was canceled. Stopping extrinsic monitoring")

		case status := <-sub.Chan():
			if status.IsDropped || status.IsInvalid {
				// Indicates that the extrinsic wasn't processed. We expect the Substrate txpool to be
				// stuck until this nonce is successfully provided. But it might be provided without this
				// relayer's intervention, e.g. if an internal Substrate mechanism re-introduces it to the
				// txpool.
				sub.Unsubscribe()
				statusStr := getStatusString(&status)
				ep.log.WithFields(logrus.Fields{
					"nonce":  nonce,
					"status": statusStr,
				}).Debug("Extrinsic failed to be processed")

				// Back off to give the txpool time to resolve any backlog
				time.Sleep(ep.getRetryDelay(nonce))

				ep.Lock()
				if nonce <= ep.maxNonce {
					// We're in the clear - no need to retry
					ep.watched--
					ep.Unlock()
					return nil
				}
				ep.Unlock()

				// This might fail because the transaction has been temporarily banned in Substrate. In that
				// case it's best to crash, wait a while and restart the relayer.
				ep.log.WithFields(logrus.Fields{
					"nonce":  nonce,
					"status": statusStr,
				}).Debug("Re-submitting failed extrinsic")
				newSub, err := ep.conn.api.RPC.Author.SubmitAndWatchExtrinsic(*ext)
				if err != nil {
					return err
				}
				sub = newSub

			} else if !status.IsReady && !status.IsFuture && !status.IsBroadcast {
				// We assume all other status codes indicate that the extrinsic was processed
				// and account nonce was incremented.
				// See details at:
				// https://github.com/paritytech/substrate/blob/29aca981db5e8bf8b5538e6c7920ded917013ef3/primitives/transaction-pool/src/pool.rs#L56-L127
				sub.Unsubscribe()
				ep.Lock()
				defer ep.Unlock()
				if nonce > ep.maxNonce {
					ep.maxNonce = nonce
				}
				ep.watched--
				return nil
			}

		case err := <-sub.Err():
			return err
		}
	}
}

func (ep *extrinsicPool) getRetryDelay(nonce uint32) time.Duration {
	ep.Lock()
	defer ep.Unlock()
	if nonce <= ep.maxNonce {
		// No delay because we don't need to retry
		return 0
	}

	// Stagger retries in the case that we have a series of failed extrinsics
	noncesToCatchUp := nonce - ep.maxNonce
	return time.Duration(noncesToCatchUp) * time.Second * 5
}

func getStatusString(status *types.ExtrinsicStatus) string {
	statusBytes, err := status.MarshalJSON()
	if err != nil {
		return "failed to serialize status"
	}
	return string(statusBytes)
}
