package storage

import (
	"github.com/bazo-blockchain/bazo-miner/protocol"
)


func UpdateBlocksToBlocksWithoutTx(block *protocol.Block) (err error){

	if rand1() {
		block.Aggregated = true
		logger.Printf("UPDATE: Write (%x) into emptyBlockBucket as (%x)", block.Hash[0:8], block.HashWithoutTx[0:8])
		WriteClosedBlockWithoutTx(block)
		DeleteClosedBlock(block.Hash)
		return err
	}
	return
}

func rand1() bool {
	//return rand.Float32() < 0.5
	return true
}
