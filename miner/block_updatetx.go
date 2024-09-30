package miner

import (
	"errors"
	"fmt"
	"github.com/way365/bazo-miner/crypto"
	"github.com/way365/bazo-miner/p2p"
	"github.com/way365/bazo-miner/protocol"
	"github.com/way365/bazo-miner/storage"
	"time"
)

// Handles the processing of a UpdateTx.
// Updates the tx to delete referenced by tx.TxToUpdateHash from the
// local storage and increases the update counter from the block it was included in.
// Adds the UpdateTx to the UpdateTxData slice of the current block.
func addUpdateTx(b *protocol.Block, tx *protocol.UpdateTx) error {

	// First we perform the update of the tx we want to update. 在updateTx上链前就修改了？
	err := handleTxUpdate(tx)
	if err != nil {
		return err
	}

	// Then we can include the UpdateTx in the current block.
	b.UpdateTxData = append(b.UpdateTxData, tx.Hash())

	return nil
}

func handleTxUpdate(updateTx *protocol.UpdateTx) error {

	txToUpdateHash := updateTx.TxToUpdateHash

	// At this point we already verified that the transaction we want to update actually exists
	// either in the open or closed transaction storage. Thus we can safely assume it exists and
	// update it in our local storage.
	updateLocalTx(txToUpdateHash, updateTx.TxToUpdateCheckString, updateTx.TxToUpdateData, updateTx.Data)

	blockToUpdate := storage.ReadBlockByTxHash(txToUpdateHash)
	if blockToUpdate == nil {
		return errors.New(fmt.Sprintf("Can't find block of tx: %x", txToUpdateHash))
	}

	//0000000000000000000000000000000000000000000000000000000000000000
	blockToUpdate.NrUpdates++

	logger.Printf("\nUpdated Block:\n%s", blockToUpdate.String())

	// Update the block in the local storage.
	storage.DeleteOpenBlock(blockToUpdate.Hash)
	storage.WriteClosedBlock(blockToUpdate)

	logger.Print("\n ------------------handleTxUpdate start broadcastBlock-----------------\n")
	go broadcastBlock(blockToUpdate) //因为Block上只保存了交易哈希，所以只需要变NrUpdates

	return nil
}

// Updates the data field of a local tx identified by txHash
func updateLocalTx(
	txHash [32]byte,
	newCheckString *crypto.ChameleonHashCheckString,
	newData []byte,
	updateReason []byte, // This is the data field from update tx.
) error {
	var txToUpdate protocol.Transaction
	var oldData []byte

	switch true {
	case storage.ReadOpenTx(txHash) != nil:
		txToUpdate = storage.ReadOpenTx(txHash) //OpenTx 是指还未被区块链网络确认并包含在区块中的交易。
		oldData = txToUpdate.GetData()
		txToUpdate.SetData(newData)
		txToUpdate.SetCheckString(newCheckString)
		storage.WriteOpenTx(txToUpdate)

	case storage.ReadClosedTx(txHash) != nil:
		txToUpdate = storage.ReadClosedTx(txHash)
		oldData = txToUpdate.GetData()
		txToUpdate.SetData(newData)
		txToUpdate.SetCheckString(newCheckString)
		storage.WriteClosedTx(txToUpdate) //ClosedTx 是指已经被区块链网络确认并写入到区块中的交易。

	default: // If we don't find the tx to update in the storage, we also can't update it.

		return errors.New(fmt.Sprintf("Can't find TxToDelete: %x", txHash))
	}

	logger.Printf("\n"+
		"=====================================================================================\n"+
		"      Updated TX: %x\n"+
		"         Old:  %s\n"+
		"         New:  %s\n\n"+
		"         Reason:  %s\n"+
		"=====================================================================================",
		txToUpdate.Hash(), oldData, txToUpdate.GetData(), updateReason,
	)

	return nil
}

// Fetch UpdateTxData  没有更新本地的交易
func fetchUpdateTxData(block *protocol.Block, updateTxSlice []*protocol.UpdateTx, initialSetup bool, errChan chan error) {
	for i, txHash := range block.UpdateTxData {
		var tx protocol.Transaction
		var updateTx *protocol.UpdateTx

		closedTx := storage.ReadClosedTx(txHash)
		if closedTx != nil {
			logger.Printf("Tx was in closed")
			if initialSetup {
				updateTx = closedTx.(*protocol.UpdateTx)
				updateTxSlice[i] = updateTx
				continue
			} else {
				//Reject blocks that have txs which have already been validated.
				errChan <- errors.New("Block validation had updateTx that was already in a previous block.")
				return
			}
		}

		//Tx is either in open storage or needs to be fetched from the network.
		tx = storage.ReadOpenTx(txHash)
		if tx != nil {
			logger.Printf("Tx was in open")
			updateTx = tx.(*protocol.UpdateTx)
		} else {
			err := p2p.TxReq(txHash, p2p.UPDATETX_REQ)
			if err != nil {
				errChan <- errors.New(fmt.Sprintf("UpdateTx could not be read: %v", err))
				return
			}

			//Blocking Wait
			select {
			case updateTx = <-p2p.UpdateTxChan:
				logger.Printf("Tx was fetched from network")
			case <-time.After(TXFETCH_TIMEOUT * time.Second):
				errChan <- errors.New("UpdateTx fetch timed out.")
			}
			//This check is important. A malicious miner might have sent us a tx whose hash is a different one
			//from what we requested.
			if updateTx.Hash() != txHash {
				errChan <- errors.New("Received UpdateTx hash did not correspond to our request.")
			}
		}

		updateTxSlice[i] = updateTx
	}

	errChan <- nil
}
