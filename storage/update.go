package storage

import (
	"github.com/bazo-blockchain/bazo-miner/protocol"
)


func UpdateBlocksToBlocksWithoutTx(block *protocol.Block) (err error){

	WriteClosedBlockWithoutTx(block)
	DeleteClosedBlock(block.Hash)
	return err
}
