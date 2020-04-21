package miner

import (
	"errors"
	"fmt"
	"github.com/julwil/bazo-miner/p2p"
	"github.com/julwil/bazo-miner/protocol"
	"github.com/julwil/bazo-miner/storage"
	"golang.org/x/crypto/sha3"
	"time"
)

func addAccTx(b *protocol.Block, tx *protocol.AccTx) error {
	accHash := sha3.Sum256(tx.PubKey[:])
	//According to the accTx specification, we only accept new accounts except if the removal bit is
	//set in the header (2nd bit).
	if tx.Header&0x02 != 0x02 {
		if _, exists := storage.State[accHash]; exists {
			return errors.New("Account already exists.")
		}
	}

	//Add the tx hash to the block header and write it to open storage (non-validated transactions).
	b.AccTxData = append(b.AccTxData, tx.Hash())
	logger.Printf("Added tx (%x) to the AccTxData slice: %v", tx.Hash(), *tx)
	return nil
}

//We use slices (not maps) because order is now important.
func fetchAccTxData(block *protocol.Block, accTxSlice []*protocol.AccTx, initialSetup bool, errChan chan error) {
	for cnt, txHash := range block.AccTxData {
		var tx protocol.Transaction
		var accTx *protocol.AccTx

		closedTx := storage.ReadClosedTx(txHash)
		if closedTx != nil {
			if initialSetup {
				accTx = closedTx.(*protocol.AccTx)
				accTxSlice[cnt] = accTx
				continue
			} else {
				//Reject blocks that have txs which have already been validated.
				errChan <- errors.New("Block validation had accTx that was already in a previous block.")
				return
			}
		}

		//TODO Optimize code (duplicated)
		//Tx is either in open storage or needs to be fetched from the network.
		tx = storage.ReadOpenTx(txHash)
		if tx != nil {
			accTx = tx.(*protocol.AccTx)
		} else {
			err := p2p.TxReq(txHash, p2p.ACCTX_REQ)
			if err != nil {
				errChan <- errors.New(fmt.Sprintf("AccTx could not be read: %v", err))
				return
			}

			//Blocking Wait
			select {
			case accTx = <-p2p.AccTxChan:
				//Limit the waiting time for TXFETCH_TIMEOUT seconds.
			case <-time.After(TXFETCH_TIMEOUT * time.Second):
				errChan <- errors.New("AccTx fetch timed out.")
			}
			//This check is important. A malicious miner might have sent us a tx whose hash is a different one
			//from what we requested.
			if accTx.Hash() != txHash {
				errChan <- errors.New("Received AcctxHash did not correspond to our request.")
			}
		}

		accTxSlice[cnt] = accTx
	}

	errChan <- nil
}
