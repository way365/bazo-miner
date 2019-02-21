package storage

import (
	"github.com/bazo-blockchain/bazo-miner/protocol"
)


func UpdateBlocksToBlocksWithoutTx(block *protocol.Block) (err error){

	if BlockReadyToAggregate(block) {
		block.Aggregated = true
		logger.Printf("UPDATE: Write (%x) into emptyBlockBucket as (%x)", block.Hash[0:8], block.HashWithoutTx[0:8])
		WriteClosedBlockWithoutTx(block)
		DeleteClosedBlock(block.Hash)
		return err
	}
	return
}

func BlockReadyToAggregate(block *protocol.Block) bool {

	// If Block contains no transactions, it can be viewed as aggregated and moved to the according bucket.
	if (block.NrAggTx == 0) && (block.NrStakeTx == 0) && (block.NrFundsTx == 0) && (block.NrAccTx == 0) && (block.NrConfigTx == 0) {
		return true
	}

	//If block has AccTx, StakeTx or ConfigTx included, it will never be aggregated.
	if (block.NrStakeTx > 0) && (block.NrAccTx > 0) && (block.NrConfigTx > 0) {
		return false
	}

	//Check if all FundsTransactions are aggregated. If not, block cannot be moved to the empty blocks bucket.
	for _, txHash := range block.FundsTxData {
		tx := ReadClosedTx(txHash).(*protocol.FundsTx)

		if tx.Aggregated == false {
			logger.Printf("Transaction (%x) not aggregated Yet.", txHash[0:8])
			return false
		}
		logger.Printf("Transaction (%x) aggregated.", txHash[0:8])
	}

	logger.Printf("All Funds Transactions in %x aggregated", block.Hash[0:8])



	//TODO Same check for aggTx is needed. Tey can also be aggregated.

	return true
}
