package miner

import (
	"errors"
	"fmt"
	"github.com/way365/bazo-miner/p2p"
	"github.com/way365/bazo-miner/protocol"
	"github.com/way365/bazo-miner/storage"
	"sync"
	"time"
)

// Function to give a list of blocks to rollback (in the right order) and a list of blocks to validate.
// Covers both cases (if block belongs to the longest chain or not).
func getBlockSequences(newBlock *protocol.Block) (blocksToRollback, blocksToValidate []*protocol.Block, err error) {
	//Fetch all blocks that are needed to validate.
	ancestor, newChain := getNewChain(newBlock)

	//Common ancestor not found, discard block.
	if ancestor == nil {
		return nil, nil, errors.New("Common ancestor not found.")
	}

	//Count how many blocks there are on the currently active chain.
	tmpBlock := lastBlock

	if tmpBlock == nil {
		tmpBlock = storage.ReadLastClosedBlock()
	}

	if tmpBlock == nil {
		return nil, nil, errors.New("Last Block not found")
	}

	var blocksToRollbackMutex = sync.Mutex{}
	for {
		blocksToRollbackMutex.Lock()
		if tmpBlock.Hash == ancestor.Hash || tmpBlock.HashWithoutTx == ancestor.HashWithoutTx {
			break
		}
		blocksToRollback = append(blocksToRollback, tmpBlock)
		//The block needs to be in closed storage.
		newTmpBlock := storage.ReadClosedBlock(tmpBlock.PrevHash)

		//Search in blocks withoutTx.
		if newTmpBlock == nil {
			newTmpBlock = storage.ReadClosedBlockWithoutTx(tmpBlock.PrevHashWithoutTx)
		}
		if newTmpBlock == nil {
			logger.Printf("Block not found: %x, %x", tmpBlock.Hash, tmpBlock.HashWithoutTx)
			blocksToRollbackMutex.Unlock()
			return nil, nil, errors.New(fmt.Sprintf("Block not found in both closed storages"))
		}
		tmpBlock = newTmpBlock
		blocksToRollbackMutex.Unlock()
	}

	//Compare current length with new chain length.
	if len(blocksToRollback) >= len(newChain) {
		//Current chain length is longer or equal (our consensus protocol states that in this case we reject the block).
		return nil, nil, errors.New(fmt.Sprintf("Block belongs to shorter or equally long chain --> NO Rollback (blocks to rollback %d vs block of new chain %d)", len(blocksToRollback), len(newChain)))
	} else {
		//New chain is longer, rollback and validate new chain.
		if len(blocksToRollback) != 0 {

			logger.Printf("Rollback (blocks to rollback %d vs block of new chain %d)", len(blocksToRollback), len(newChain))
			logger.Printf("ANCESTOR: %x", ancestor.Hash[0:8])

		}
		return blocksToRollback, newChain, nil
	}
}

// Returns the ancestor from which the split occurs (if a split occurred, if not it's just our last block) and a list
// of blocks that belong to a new chain.
func getNewChain(newBlock *protocol.Block) (ancestor *protocol.Block, newChain []*protocol.Block) {
	found := false
	for {
		newChain = append(newChain, newBlock)

		//Search for an ancestor (which needs to be in closed storage -> validated block).
		//Search in closed (Validated) blocks first
		potentialAncestor := storage.ReadClosedBlock(newBlock.PrevHash)
		if potentialAncestor != nil {
			//Found ancestor because it is found in our closed block storage.
			//We went back in time, so reverse order.
			newChain = InvertBlockArray(newChain)
			return potentialAncestor, newChain
		}

		potentialAncestor = storage.ReadClosedBlockWithoutTx(newBlock.PrevHashWithoutTx)
		if potentialAncestor != nil {
			//Found ancestor because it is found in our closed block storage.
			//We went back in time, so reverse order.
			newChain = InvertBlockArray(newChain)
			return potentialAncestor, newChain
		}

		//It might be the case that we already started a sync and the block is in the openblock storage.
		openBlock := storage.ReadOpenBlock(newBlock.PrevHash)
		if openBlock != nil {
			continue
		}

		// Check if block is in received stash. When in there, continue outer for-loop, until ancestor
		// is found in closed block storage. The blocks from the stash will be validated in the normal validation process
		// after the rollback. (Similar like when in open storage) If not in stash, continue with a block request to
		// the network. Keep block in stash in case of multiple rollbacks (Very rare)
		for _, block := range storage.ReadReceivedBlockStash() {
			if block.Hash == newBlock.PrevHash {
				newBlock = block
				found = true
				break
			}
		}

		if found {
			found = false
			continue
		}

		//Fetch the block we apparently missed from the network.
		//p2p.BlockReq(newBlock.PrevHash, newBlock.PrevHashWithoutTx)
		requestHash := newBlock.PrevHash
		requestHashWithoutTx := newBlock.PrevHashWithoutTx
		p2p.BlockReq(requestHash, requestHashWithoutTx)

		//Blocking wait
		select {
		case encodedBlock := <-p2p.BlockReqChan:
			newBlock = newBlock.Decode(encodedBlock)
			storage.WriteToReceivedStash(newBlock)
		//Limit waiting time to BLOCKFETCH_TIMEOUT seconds before aborting.
		case <-time.After(BLOCKFETCH_TIMEOUT * time.Second):
			logger.Printf("Timed Out fetching %x in longestChain -> Search in received Block stash", requestHash)
			if p2p.BlockAlreadyReceived(storage.ReadReceivedBlockStash(), requestHash) {
				for _, block := range storage.ReadReceivedBlockStash() {
					if block.Hash == requestHash {
						newBlock = block
						logger.Printf("Block %x was in Received Block Stash", requestHash)
						break
					}
				}
				break
			}
			return nil, nil
		}
	}

	return nil, nil
}
