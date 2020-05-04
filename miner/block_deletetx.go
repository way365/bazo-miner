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
	//handleTxDeletion(tx.TxToDeleteHash)

	// Then we can include the DeleteTx in the current block.
	b.DeleteTxData = append(b.DeleteTxData, tx.Hash())

	return nil
}

// Handles the deletion of a given tx
func handleTxDeletion(txHash [32]byte) error {

	// At this point we already verified that the transaction we want to delete actually exists
	// either in the open or closed transaction storage. Thus we can safely assume it exists and
	// delete it from our local storage.
	deleteTxFromStorage(txHash)

	// We also need to remove the hash of the tx from the block it was included in, when tx was initially mined.
	// First we get the block form the local storage where the tx was included in.
	blockToUpdate := storage.ReadBlockByTxHash(txHash)
	if blockToUpdate == nil {
		return errors.New(fmt.Sprintf("Can't find block of tx: %x", txHash))
	}

	// Then we remove the tx from the block slice.
	updatedBlock, err := deleteTxFromBlock(txHash, blockToUpdate)
	if err != nil {
		return errors.New(fmt.Sprintf("\nRemoving \ntx: %x from \nblock: %x failed.", txHash, blockToUpdate.Hash))
	}

	logger.Printf("\nUpdated Block:\n%s", updatedBlock.String())

	// Now we process the updated block like any other.

	return nil
}

// Deletes a given tx from the local storage
func deleteTxFromStorage(txHash [32]byte) error {
	var txToDelete protocol.Transaction

	switch true {
	case storage.ReadOpenTx(txHash) != nil:
		txToDelete = storage.ReadOpenTx(txHash)
		storage.DeleteOpenTx(txToDelete)

		if storage.ReadOpenTx(txHash) == nil {
			logger.Printf("\nTx: %x was deleted from open transaction storage.", txHash)
		}

	case storage.ReadClosedTx(txHash) != nil:
		txToDelete = storage.ReadClosedTx(txHash)
		storage.DeleteClosedTx(txToDelete)

		if storage.ReadClosedTx(txHash) == nil {
			logger.Printf("\nTx: %x was deleted from closed transaction storage.", txHash)
		}

	default: // If we don't find the tx to delete in the storage, we also can't delete it.

		return errors.New(fmt.Sprintf("Can't find TxToDelete: %x", txHash))
	}

	return nil
}

// Deletes a given tx from the block it was included in.
func deleteTxFromBlock(txHash [32]byte, block *protocol.Block) (*protocol.Block, error) {

	// First we remove the txHash from the respective block slice and
	// decrease the respective tx counter.
	txInBlock := true
	for txInBlock {

		// Agg
		for i, aggTxHash := range block.AggTxData {
			if aggTxHash == txHash {
				block.AggTxData = remove(block.AggTxData, i)
				logger.Printf("\nLocated tx in AggTxData block slice")
				txInBlock = false
				break
			}
		}

		// Funds
		for i, fundsTxHash := range block.FundsTxData {
			if fundsTxHash == txHash {
				newFundsTxData := remove(block.FundsTxData, i)

				// If the update tx slice is empty
				if len(newFundsTxData) == 0 {
					newFundsTxData = nil
				}

				block.FundsTxData = newFundsTxData
				block.NrFundsTx--
				txInBlock = false
				break
			}
		}

		// Accounts
		for i, accTxHash := range block.AccTxData {
			if accTxHash == txHash {
				newAccTxData := remove(block.AccTxData, i)

				// If the update tx slice is empty
				if len(newAccTxData) == 0 {
					newAccTxData = nil
				}

				block.FundsTxData = newAccTxData
				block.NrAccTx--
				txInBlock = false
				break
			}
		}

		// Config
		for i, configTxHash := range block.ConfigTxData {
			if configTxHash == txHash {
				newConfigTxData := remove(block.ConfigTxData, i)

				// If the update tx slice is empty
				if len(newConfigTxData) == 0 {
					newConfigTxData = nil
				}

				block.FundsTxData = newConfigTxData
				block.NrConfigTx--
				txInBlock = false
				break
			}
		}

		// Delete
		for i, deleteTxHash := range block.DeleteTxData {
			if deleteTxHash == txHash {
				newDeleteTxData := remove(block.DeleteTxData, i)

				// If the update tx slice is empty
				if len(newDeleteTxData) == 0 {
					newDeleteTxData = nil
				}

				block.FundsTxData = newDeleteTxData
				block.NrDeleteTx--
				txInBlock = false
				break
			}
		}
	}

	// Then we re-hash the block based on the updated content //TODO: This should break the BC --> apply CHF here
	logger.Printf("\nUpdated block:\n%v", block.String())

	return block, nil
}

// Removes item at index i from the slice s.
// Returns the updated slice.
func remove(slice [][32]byte, i int) [][32]byte {
	slice[len(slice)-1], slice[i] = slice[i], slice[len(slice)-1]

	return slice[:len(slice)-1]
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
