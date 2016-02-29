package fedchain

import (
	"time"

	"golang.org/x/net/context"

	"chain/errors"
	"chain/fedchain/bc"
	"chain/fedchain/state"
	"chain/fedchain/validation"
	"chain/metrics"
)

// AddTx inserts tx into the set of "pending" transactions available
// to be included in the next block produced by GenerateBlock.
//
// It validates tx against the blockchain state and the existing
// pending pool.
//
// It is okay to add the same transaction more than once; subsequent
// attempts will have no effect and return a nil error.
//
// TODO(kr): accept tx if it is valid for any *subset* of the pool.
// This means accepting conflicting transactions in the same pool
// at the same time.
func (fc *FC) AddTx(ctx context.Context, tx *bc.Tx) error {
	poolView, err := fc.store.NewPoolViewForPrevouts(ctx, []*bc.Tx{tx})
	if err != nil {
		return errors.Wrap(err)
	}

	bcView, err := fc.store.NewViewForPrevouts(ctx, []*bc.Tx{tx})
	if err != nil {
		return errors.Wrap(err)
	}

	// Check if the transaction already exists in the blockchain.
	txs, err := fc.store.GetTxs(ctx, tx.Hash)
	if _, ok := txs[tx.Hash]; ok {
		return nil
	}
	if err != nil {
		return errors.Wrap(err)
	}

	view := state.MultiReader(poolView, bcView)
	err = validation.ValidateTx(ctx, view, tx, uint64(time.Now().Unix()))
	if err != nil {
		return errors.Wrapf(ErrTxRejected, "validate tx: %v", err)
	}

	// Update persistent tx pool state
	err = fc.applyTx(ctx, tx, sumIssued(ctx, view, tx))
	if err != nil {
		return errors.Wrap(err, "apply TX")
	}

	for _, cb := range fc.txCallbacks {
		cb(ctx, tx)
	}
	return nil
}

// applyTx updates the output set to reflect
// the effects of tx. It deletes consumed utxos
// and inserts newly-created outputs.
// Must be called inside a transaction.
func (fc *FC) applyTx(ctx context.Context, tx *bc.Tx, issued map[bc.AssetID]uint64) (err error) {
	defer metrics.RecordElapsed(time.Now())

	err = fc.store.ApplyTx(ctx, tx, issued)
	return errors.Wrap(err, "applying tx to store")
}

// the amount of issued assets can be determined by
// the sum of outputs minus the sum of non-issuance inputs
func sumIssued(ctx context.Context, view state.ViewReader, txs ...*bc.Tx) map[bc.AssetID]uint64 {
	issued := make(map[bc.AssetID]uint64)
	for _, tx := range txs {
		if !tx.HasIssuance() {
			continue
		}
		for _, out := range tx.Outputs {
			issued[out.AssetID] += out.Amount
		}
		for _, in := range tx.Inputs {
			if in.IsIssuance() {
				continue
			}
			prevout := view.Output(ctx, in.Previous)
			issued[prevout.AssetID] -= prevout.Amount
		}
	}
	return issued
}
