package miner

import (
	"errors"
	"fmt"
	"github.com/way365/bazo-miner/p2p"
	"github.com/way365/bazo-miner/protocol"
	"github.com/way365/bazo-miner/storage"
	"time"
)

func addConfigTx(b *protocol.Block, tx *protocol.ConfigTx) error {
	//No further checks needed, static checks were already done with verify().
	b.ConfigTxData = append(b.ConfigTxData, tx.Hash())
	logger.Printf("Added tx (%x) to the ConfigTxData slice: %v", tx.Hash(), *tx)
	return nil
}

func fetchConfigTxData(block *protocol.Block, configTxSlice []*protocol.ConfigTx, initialSetup bool, errChan chan error) {
	for cnt, txHash := range block.ConfigTxData {
		var tx protocol.Transaction
		var configTx *protocol.ConfigTx

		closedTx := storage.ReadClosedTx(txHash)
		if closedTx != nil {
			if initialSetup {
				configTx = closedTx.(*protocol.ConfigTx)
				configTxSlice[cnt] = configTx
				continue
			} else {
				errChan <- errors.New("Block validation had configTx that was already in a previous block.")
				return
			}
		}

		//TODO Optimize code (duplicated)
		tx = storage.ReadOpenTx(txHash)
		if tx != nil {
			configTx = tx.(*protocol.ConfigTx)
		} else {
			err := p2p.TxReq(txHash, p2p.CONFIGTX_REQ)
			if err != nil {
				errChan <- errors.New(fmt.Sprintf("ConfigTx could not be read: %v", err))
				return
			}

			select {
			case configTx = <-p2p.ConfigTxChan:
			case <-time.After(TXFETCH_TIMEOUT * time.Second):
				errChan <- errors.New("ConfigTx fetch timed out.")
				return
			}
			if configTx.Hash() != txHash {
				errChan <- errors.New("Received ConfigtxHash did not correspond to our request.")
			}
		}

		configTxSlice[cnt] = configTx
	}

	errChan <- nil
}
