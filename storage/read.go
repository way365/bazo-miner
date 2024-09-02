package storage

import (
	"github.com/boltdb/bolt"
	"github.com/way365/bazo-miner/protocol"
	"sort"
)

// Always return nil if requested hash is not in the storage. This return value is then checked against by the caller
func ReadOpenBlock(hash [32]byte) (block *protocol.Block) {

	var encodedBlock []byte
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("openblocks"))
		encodedBlock = b.Get(hash[:])
		return nil
	})

	if encodedBlock == nil {
		return nil
	}

	return block.Decode(encodedBlock)
}

func ReadClosedBlock(hash [32]byte) (block *protocol.Block) {

	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("closedblocks"))
		encodedBlock := b.Get(hash[:])
		block = block.Decode(encodedBlock)
		return nil
	})

	if block == nil {
		return nil
	}

	return block
}

// This function does read all blocks without transactions inside.
func ReadClosedBlockWithoutTx(hash [32]byte) (block *protocol.Block) {

	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("closedblockswithouttx"))
		encodedBlock := b.Get(hash[:])
		block = block.Decode(encodedBlock)
		return nil
	})

	if block == nil {
		return nil
	}

	return block
}

func ReadLastClosedBlock() (block *protocol.Block) {

	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("lastclosedblock"))
		cb := b.Cursor()
		_, encodedBlock := cb.First()
		block = block.Decode(encodedBlock)
		return nil
	})

	if block == nil {
		return nil
	}

	return block
}

func ReadAllClosedBlocksWithTransactions() (allClosedBlocks []*protocol.Block) {

	//This does return all blocks which are either in closedblocks or closedblockswithouttx bucket of the Database.
	//They are not ordered at teh request, but this does actually not matter. Because it will be ordered below
	block := ReadLastClosedBlock()
	if block != nil {
		db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("closedblocks"))
			b.ForEach(func(k, v []byte) error {
				if v != nil {
					encodedBlock := v
					block = block.Decode(encodedBlock)
					allClosedBlocks = append(allClosedBlocks, block)
				}
				return nil
			})
			return nil
		})
	}

	//blocks are sorted here.
	sort.Sort(ByHeight(allClosedBlocks))

	return allClosedBlocks
}

func ReadAllClosedFundsAndAggTransactions() (allClosedTransactions []protocol.Transaction) {

	var fundsTx protocol.FundsTx
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("closedfunds"))
		b.ForEach(func(k, v []byte) error {
			if v != nil {
				encodedFundsTx := v
				allClosedTransactions = append(allClosedTransactions, fundsTx.Decode(encodedFundsTx))
			}
			return nil
		})
		return nil
	})

	var aggTx protocol.AggTx
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("closedaggregations"))
		b.ForEach(func(k, v []byte) error {
			if v != nil {
				encodedAggTx := v
				if encodedAggTx != nil {
					allClosedTransactions = append(allClosedTransactions, aggTx.Decode(encodedAggTx))
				}
			}
			return nil
		})
		return nil
	})

	return allClosedTransactions
}

// This method does read all blocks in closedBlocks & closedblockswithouttx.
func ReadAllClosedBlocks() (allClosedBlocks []*protocol.Block) {

	//This does return all blocks which are either in closedblocks or closedblockswithouttx bucket of the Database.
	//They are not ordered at teh request, but this does actually not matter. Because it will be ordered below
	block := ReadLastClosedBlock()
	if block != nil {
		db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("closedblocks"))
			b.ForEach(func(k, v []byte) error {
				if v != nil {
					encodedBlock := v
					block = block.Decode(encodedBlock)
					allClosedBlocks = append(allClosedBlocks, block)
				}
				return nil
			})

			b = tx.Bucket([]byte("closedblockswithouttx"))
			b.ForEach(func(k, v []byte) error {
				if v != nil {
					encodedBlock := v
					block = block.Decode(encodedBlock)
					allClosedBlocks = append(allClosedBlocks, block)
				}
				return nil
			})

			return nil
		})
	}

	//blocks are sorted here.
	sort.Sort(ByHeight(allClosedBlocks))

	return allClosedBlocks
}

// The three functions and the type below are used to order the gathered closed blocks from below according to
// their block height.
type ByHeight []*protocol.Block

func (a ByHeight) Len() int           { return len(a) }
func (a ByHeight) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByHeight) Less(i, j int) bool { return a[i].Height < a[j].Height }

func ReadReceivedBlockStash() (receivedBlocks []*protocol.Block) {
	return ReceivedBlockStash
}

func ReadOpenTx(hash [32]byte) (transaction protocol.Transaction) {
	openTxMutex.Lock()
	defer openTxMutex.Unlock()
	return txMemPool[hash]
}

func ReadTxcntToTx(txCnt uint32) (transactions [][32]byte) {
	txcntToTxMapMutex.Lock()
	defer txcntToTxMapMutex.Unlock()
	return TxcntToTxMap[txCnt]
}

func ReadFundsTxBeforeAggregation() []*protocol.FundsTx {
	openFundsTxBeforeAggregationMutex.Lock()
	defer openFundsTxBeforeAggregationMutex.Unlock()
	return FundsTxBeforeAggregation
}

func ReadAllBootstrapReceivedTransactions() (allOpenTxs []protocol.Transaction) {
	openTxMutex.Lock()
	defer openTxMutex.Unlock()

	for key := range bootstrapReceivedMemPool {
		allOpenTxs = append(allOpenTxs, txMemPool[key])
	}
	return
}

func ReadINVALIDOpenTx(hash [32]byte) (transaction protocol.Transaction) {
	openINVALIDTxMutex.Lock()
	defer openINVALIDTxMutex.Unlock()
	return txINVALIDMemPool[hash]
}

func ReadAllINVALIDOpenTx() (allOpenInvalidTxs []protocol.Transaction) {

	openINVALIDTxMutex.Lock()
	defer openINVALIDTxMutex.Unlock()
	for key := range txINVALIDMemPool {
		allOpenInvalidTxs = append(allOpenInvalidTxs, txINVALIDMemPool[key])
	}

	return allOpenInvalidTxs
}

// Needed for the miner to prepare a new block
func ReadAllOpenTxs() (allOpenTxs []protocol.Transaction) {
	openTxMutex.Lock()
	defer openTxMutex.Unlock()

	for key := range txMemPool {
		allOpenTxs = append(allOpenTxs, txMemPool[key])
	}
	return
}

// Personally I like it better to test (which tx type it is) here, and get returned the interface. Simplifies the code
func ReadClosedTx(hash [32]byte) (transaction protocol.Transaction) {
	var encodedTx []byte
	var fundstx *protocol.FundsTx
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("closedfunds"))
		encodedTx = b.Get(hash[:])
		return nil
	})
	if encodedTx != nil {
		return fundstx.Decode(encodedTx)
	}

	var acctx *protocol.AccTx
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("closedaccs"))
		encodedTx = b.Get(hash[:])
		return nil
	})
	if encodedTx != nil {
		return acctx.Decode(encodedTx)
	}

	var updateTx *protocol.UpdateTx
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("closeddeletes"))
		encodedTx = b.Get(hash[:])
		return nil
	})
	if encodedTx != nil {
		return updateTx.Decode(encodedTx)
	}

	var configtx *protocol.ConfigTx
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("closedconfigs"))
		encodedTx = b.Get(hash[:])
		return nil
	})
	if encodedTx != nil {
		return configtx.Decode(encodedTx)
	}

	var staketx *protocol.StakeTx
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("closedstakes"))
		encodedTx = b.Get(hash[:])
		return nil
	})
	if encodedTx != nil {
		return staketx.Decode(encodedTx)
	}

	var aggTx *protocol.AggTx
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("closedaggregations"))
		encodedTx = b.Get(hash[:])
		return nil
	})
	if encodedTx != nil {
		return aggTx.Decode(encodedTx)
	}

	return nil
}

// Returns the block which contains the given tx hash.
func ReadBlockByTxHash(txHash [32]byte) (block *protocol.Block) {

	var blockHash [32]byte
	db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("blocksbytxhash"))
		copy(blockHash[:], bucket.Get(txHash[:]))
		return nil
	})

	switch true {
	case ReadOpenBlock(blockHash) != nil:
		return ReadOpenBlock(blockHash)

	case ReadClosedBlock(blockHash) != nil:
		return ReadClosedBlock(blockHash)

	default:
		return nil
	}
}
