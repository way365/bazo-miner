package miner

import (
	"errors"
	"fmt"
	"github.com/julwil/bazo-miner/p2p"
	"github.com/julwil/bazo-miner/protocol"
	"github.com/julwil/bazo-miner/storage"
	"time"
)

// Handles the processing of a DeleteTx.
// Deletes the tx to delete referenced by tx.TxToDeleteHash from the
// local storage and removes the hash from the tx from the block it was included in.
// Adds the DeleteTx to the DeleteTxData slice of the current block.
func addDeleteTx(b *protocol.Block, tx *protocol.DeleteTx) error {

	// First we perform the deletion of the tx we want to remove.
	handleTxDeletion(tx)

	// Then we can include the DeleteTx in the current block.
	b.DeleteTxData = append(b.DeleteTxData, tx.Hash())

	return nil
}

// Handles the processing of a given DeleteTx
func handleTxDeletion(deleteTx *protocol.DeleteTx) error {

	txToDeleteHash := deleteTx.TxToDeleteHash
	//deleteTxHash := deleteTx.Hash()

	// At this point we already verified that the transaction we want to delete actually exists
	// either in the open or closed transaction storage. Thus we can safely assume it exists and
	// delete it from our local storage.
	removeDataFromLocalTx(txToDeleteHash)

	blockToUpdate := storage.ReadBlockByTxHash(txToDeleteHash) // TODO implement a linear scan on blocks to locate the blockToUpdate. The ReadBlockByTxHash method is only here for convenience and will be removed in the future.
	if blockToUpdate == nil {
		return errors.New(fmt.Sprintf("Can't find block of tx: %x", txToDeleteHash))
	}

	blockToUpdate.NrUpdates++

	logger.Printf("\nUpdated Block:\n%s", blockToUpdate.String())

	// Update the block in the local storage.
	storage.DeleteOpenBlock(blockToUpdate.Hash)
	storage.WriteClosedBlock(blockToUpdate)

	go broadcastBlock(blockToUpdate)
	logger.Printf("\nBroadcasted updated Block:\n%s", blockToUpdate.String())

	return nil
}

// Clears the data field of a local tx identified by txHash
func removeDataFromLocalTx(txHash [32]byte) error {
	var txToUpdate protocol.Transaction

	switch true {
	case storage.ReadOpenTx(txHash) != nil:
		txToUpdate = storage.ReadOpenTx(txHash)
		txToUpdate.SetData([]byte{}) // Set the data field to an empty slice.
		storage.WriteOpenTx(txToUpdate)

	case storage.ReadClosedTx(txHash) != nil:
		txToUpdate = storage.ReadClosedTx(txHash)
		txToUpdate.SetData([]byte{}) // Set the data field to an empty slice.
		storage.WriteClosedTx(txToUpdate)

	default: // If we don't find the tx to delete in the storage, we also can't delete it.

		return errors.New(fmt.Sprintf("Can't find TxToDelete: %x", txHash))
	}

	logger.Printf("Removed data from %s. Cleaned tx:%s", txToUpdate.Hash(), txToUpdate.String())

	return nil
}

// Fetch DeleteTxData
func fetchDeleteTxData(block *protocol.Block, deleteTxSlice []*protocol.DeleteTx, initialSetup bool, errChan chan error) {
	for i, txHash := range block.DeleteTxData {
		var tx protocol.Transaction
		var deleteTx *protocol.DeleteTx

		closedTx := storage.ReadClosedTx(txHash)
		if closedTx != nil {
			logger.Printf("Tx was in closed")
			if initialSetup {
				deleteTx = closedTx.(*protocol.DeleteTx)
				deleteTxSlice[i] = deleteTx
				continue
			} else {
				//Reject blocks that have txs which have already been validated.
				errChan <- errors.New("Block validation had deleteTx that was already in a previous block.")
				return
			}
		}

		//Tx is either in open storage or needs to be fetched from the network.
		tx = storage.ReadOpenTx(txHash)
		if tx != nil {
			logger.Printf("Tx was in open")
			deleteTx = tx.(*protocol.DeleteTx)
		} else {
			err := p2p.TxReq(txHash, p2p.DELTX_REQ)
			if err != nil {
				errChan <- errors.New(fmt.Sprintf("DeleteTx could not be read: %v", err))
				return
			}

			//Blocking Wait
			select {
			case deleteTx = <-p2p.DeleteTxChan:
				logger.Printf("Tx was fetched from network")
			case <-time.After(TXFETCH_TIMEOUT * time.Second):
				errChan <- errors.New("DeleteTx fetch timed out.")
			}
			//This check is important. A malicious miner might have sent us a tx whose hash is a different one
			//from what we requested.
			if deleteTx.Hash() != txHash {
				errChan <- errors.New("Received DeleteTxHash did not correspond to our request.")
			}
		}

		deleteTxSlice[i] = deleteTx
	}

	errChan <- nil
}
