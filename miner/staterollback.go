package miner

import (
	"github.com/bazo-blockchain/bazo-miner/protocol"
	"github.com/bazo-blockchain/bazo-miner/storage"
	"sort"
)

func accStateChangeRollback(txSlice []*protocol.AccTx) {
	for _, tx := range txSlice {
		if tx.Header == 0 || tx.Header == 1 || tx.Header == 2 {
			accHash := protocol.SerializeHashContent(tx.PubKey)

			acc, err := storage.GetAccount(accHash)
			if err != nil {
				logger.Fatal("CRITICAL: An account that should have been saved does not exist.")
			}

			delete(storage.State, accHash)

			switch tx.Header {
			case 1:
				delete(storage.RootKeys, accHash)
			case 2:
				storage.RootKeys[accHash] = acc
			}
		}
	}
}

func fundsStateChangeRollback(txSlice []*protocol.FundsTx) {
	//Rollback in reverse order than original state change
	for cnt := len(txSlice) - 1; cnt >= 0; cnt-- {
		tx := txSlice[cnt]

		accSender, _ := storage.GetAccount(tx.From)
		accReceiver, _ := storage.GetAccount(tx.To)

		accSender.TxCnt -= 1
		accSender.Balance += tx.Amount
		accReceiver.Balance -= tx.Amount

		//If new coins were issued, revert
		if rootAcc, _ := storage.GetRootAccount(tx.From); rootAcc != nil {
			rootAcc.Balance -= tx.Amount
			rootAcc.Balance -= tx.Fee
		}
	}
}

//This method does search historic blocks which do not have any transactions inside and now have to be reactivated
// because a transaction validated in this block was aggregated later in a block which now is rolled back. Thus all the
// transactions in this block need ot be reactivated such that they are visible in the chain.
func reactivateHistoricBlockDueToRollback(tx protocol.Transaction)() {
	var fundsTx *protocol.FundsTx
	var aggTx *protocol.AggTx
	var blockHash [32]byte

	switch tx.(type){
	case *protocol.FundsTx:
		fundsTx = tx.(*protocol.FundsTx)
		logger.Printf("      Need to rollback historic FundsTx: %x", fundsTx.Hash())
		blockHash = fundsTx.Block
	case *protocol.AggTx:
		aggTx = tx.(*protocol.AggTx)
		logger.Printf("      Need to rollback historic AggTx: %x", aggTx.Hash())
		blockHash = aggTx.Block
	}

	//Search block in bucket & Change aggregated to false.
	block := storage.ReadClosedBlockWithoutTx(blockHash)
	if block == nil {
		//Block is still in closed blocks but not aggregated --> only change transactions to aggregated = false such that the block does not get aggregated.
		logger.Printf("Block does still contain transactions which are not aggregated yet or is in security zone...")

		for _, tx := range storage.ReadAllClosedFundsAndAggTransactions() {
			switch tx.(type){
			case *protocol.FundsTx:
				FTX := tx.(*protocol.FundsTx)
				if FTX.Block == blockHash {
					//Reactivate this transaction --> it is still closed but not aggregated.
					logger.Printf("            Un-Aggregate fundsTx %x", FTX.Hash())
					FTX.Aggregated = false
					storage.WriteClosedTx(FTX)
				}
			case *protocol.AggTx:
				ATX := tx.(*protocol.AggTx)
				if ATX.Block == blockHash {
					//Reactivate this transaction
					logger.Printf("            Un-Aggregate aggTX %x", ATX.Hash())
					ATX.Aggregated = false
					storage.WriteClosedTx(ATX)
				}
			}
		}
		return
	}


	logger.Printf("  Found Block (%x) for which Tx's have to been found. \n", block.Hash)

	//Search all transactions which were validated in the "empty" block. This may be very time consuming.
	for _, tx := range storage.ReadAllClosedFundsAndAggTransactions() {
		switch tx.(type){
		case *protocol.FundsTx:
			FTX := tx.(*protocol.FundsTx)
			if FTX.Block == blockHash {
				//Reactivate this transaction --> it is still closed but it has to be visible in the block and chain again.
				logger.Printf("            ReActivate fundsTx %x", FTX.Hash())
				FTX.Aggregated = false
				block.FundsTxData = append(block.FundsTxData, tx.Hash())
				storage.WriteClosedTx(FTX)
			}
		case *protocol.AggTx:
			ATX := tx.(*protocol.AggTx)
			if ATX.Block == blockHash {
				//Reactivate this transaction
				logger.Printf("            ReActivate aggTX %x", ATX.Hash())
				ATX.Aggregated = false
				block.AggTxData = append(block.AggTxData, tx.Hash())
				storage.WriteClosedTx(ATX)
			}
		}
	}

	//Write block back to bucket with closed blocks with transactions.
	block.Aggregated = false
	storage.WriteClosedBlock(block)
	storage.DeleteClosedBlockWithoutTx(blockHash)
	logger.Printf("UPDATE: Rollback Write (%x) into closedBlockBucket as (%x)", block.HashWithoutTx[0:8], block.Hash[0:8])
	logger.Printf("UPDATE: Rollback %v", block)
}

func aggregatedStateRollback(txSlice []*protocol.AggTx, blockHash [32]byte) {
	//Rollback in reverse order than original state change
	var fundsTxSlice []*protocol.FundsTx
	for cnt := len(txSlice) - 1; cnt >= 0; cnt-- {
		tx := txSlice[cnt]

		logger.Printf("In tx: %x following transactions need to be rolled back: ", tx.Hash())
		//Adding all Aggregated FundsTx in reverse order.
		for _, txHash := range tx.AggregatedTxSlice {
			trx := storage.ReadClosedTx(txHash)

			switch trx.(type) {
			case *protocol.FundsTx:
				//Only rollback FundsTx which are validated in the current block.
				if trx.(*protocol.FundsTx).Block == blockHash {
					logger.Printf("  Need to rollback FundsTx: %x", trx.Hash())
					fundsTxSlice = append([]*protocol.FundsTx{trx.(*protocol.FundsTx)},fundsTxSlice...)
				} else {
					//Transaction is not validated in the current block --> Need to be reactivated in the historic block.
					logger.Printf("  Need to rollback historic Tx: %x", trx.Hash())
					reactivateHistoricBlockDueToRollback(trx)
				}
			case *protocol.AggTx:
				//AggTx can only be historic and therefore need to be reactivated in the historic block.
				reactivateHistoricBlockDueToRollback(trx)
			}
		}

		//do normal rollback for fundsTx
		logger.Printf("STATE BEFORE ROLLBACK: \n%v", getState())

		sort.Sort(ByTxCount(fundsTxSlice))

		for _, i := range fundsTxSlice {
			logger.Printf("|--- Rollback: %x, A: %v, F: %x, T: %x, cnt: %v", i.Hash(), i.Amount, i.From[0:4], i.To[0:4], i.TxCnt)
		}
		fundsStateChangeRollback(fundsTxSlice)
		logger.Printf("STATE AFTER ROLLBACK: \n%v", getState())
	}
}

func configStateChangeRollback(txSlice []*protocol.ConfigTx, blockHash [32]byte) {
	if len(txSlice) == 0 {
		return
	}

	//Only rollback if the config changes lead to a parameterChange
	//there might be the case that the client is not running the latest version, it's still confirming
	//the transaction but does not understand the ID and thus is not changing the state
	if parameterSlice[len(parameterSlice)-1].BlockHash != blockHash {
		return
	}

	//remove the latest entry in the parameters slice$
	parameterSlice = parameterSlice[:len(parameterSlice)-1]
	activeParameters = &parameterSlice[len(parameterSlice)-1]
	logger.Printf("Config parameters rolled back. New configuration: %v", *activeParameters)
}

func stakeStateChangeRollback(txSlice []*protocol.StakeTx) {
	//Rollback in reverse order than original state change
	for cnt := len(txSlice) - 1; cnt >= 0; cnt-- {
		tx := txSlice[cnt]

		accSender, _ := storage.GetAccount(tx.Account)
		//Rolling back stakingBlockHeight not needed
		accSender.IsStaking = !accSender.IsStaking
	}
}

func collectTxFeesRollback(accTx []*protocol.AccTx, fundsTx []*protocol.FundsTx, configTx []*protocol.ConfigTx, stakeTx []*protocol.StakeTx, minerHash [32]byte) {
	minerAcc, _ := storage.GetAccount(minerHash)

	//Subtract fees from sender (check if that is allowed has already been done in the block validation)
	for _, tx := range accTx {
		//Money was created out of thin air, no need to write back
		minerAcc.Balance -= tx.Fee
	}

	for _, tx := range fundsTx {
		minerAcc.Balance -= tx.Fee

		senderAcc, _ := storage.GetAccount(tx.From)
		senderAcc.Balance += tx.Fee
	}

	for _, tx := range configTx {
		//Money was created out of thin air, no need to write back
		minerAcc.Balance -= tx.Fee
	}

	for _, tx := range stakeTx {
		minerAcc.Balance -= tx.Fee

		senderAcc, _ := storage.GetAccount(tx.Account)
		senderAcc.Balance += tx.Fee
	}
}

func collectBlockRewardRollback(reward uint64, minerHash [32]byte) {
	minerAcc, _ := storage.GetAccount(minerHash)
	minerAcc.Balance -= reward
}

func collectSlashRewardRollback(reward uint64, block *protocol.Block) {
	if block.SlashedAddress != [32]byte{} || block.ConflictingBlockHash1 != [32]byte{} || block.ConflictingBlockHash2 != [32]byte{} || block.ConflictingBlockHashWithoutTx1 != [32]byte{} || block.ConflictingBlockHashWithoutTx2 != [32]byte{} {
		minerAcc, _ := storage.GetAccount(block.Beneficiary)
		slashedAcc, _ := storage.GetAccount(block.SlashedAddress)

		minerAcc.Balance -= reward
		slashedAcc.Balance += activeParameters.Staking_minimum
		slashedAcc.IsStaking = true
	}
}
