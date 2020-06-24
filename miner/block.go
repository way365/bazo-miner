package miner

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/julwil/bazo-miner/crypto"
	"github.com/julwil/bazo-miner/protocol"
	"github.com/julwil/bazo-miner/storage"
	"golang.org/x/crypto/sha3"
	"sync"
)

//Datastructure to fetch the payload of all transactions, needed for state validation.
type blockData struct {
	accTxSlice             []*protocol.AccTx
	fundsTxSlice           []*protocol.FundsTx
	configTxSlice          []*protocol.ConfigTx
	stakeTxSlice           []*protocol.StakeTx
	aggTxSlice             []*protocol.AggTx
	aggregatedFundsTxSlice []*protocol.FundsTx
	updateTxSlice          []*protocol.UpdateTx
	block                  *protocol.Block
}

//Block constructor, argument is the previous block in the blockchain.
func newBlock(prevHash [32]byte, prevHashWithoutTx [32]byte, commitmentProof [crypto.COMM_PROOF_LENGTH]byte, height uint32) *protocol.Block {
	block := new(protocol.Block)
	block.PrevHash = prevHash
	block.PrevHashWithoutTx = prevHashWithoutTx
	block.CommitmentProof = commitmentProof
	block.Height = height
	block.StateCopy = make(map[[32]byte]*protocol.Account)
	block.Aggregated = false

	return block
}

var (
	aggregationMutex = &sync.Mutex{}
	addFundsTxMutex  = &sync.Mutex{}
)

//This function prepares the block to broadcast into the network. No new txs are added at this point.
func finalizeBlock(block *protocol.Block) error {
	//Check if we have a slashing proof that we can add to the block.
	//The slashingDict is updated when a new block is received and when a slashing proof is provided.
	logger.Printf("-- Start Finalize")
	if len(slashingDict) != 0 {
		//Get the first slashing proof.
		for hash, slashingProof := range slashingDict {
			block.SlashedAddress = hash
			block.ConflictingBlockHash1 = slashingProof.ConflictingBlockHash1
			block.ConflictingBlockHash2 = slashingProof.ConflictingBlockHash2
			block.ConflictingBlockHashWithoutTx1 = slashingProof.ConflictingBlockHashWithoutTx1
			block.ConflictingBlockHashWithoutTx2 = slashingProof.ConflictingBlockHashWithoutTx2
			break
		}
	}

	//Merkle tree includes the hashes of all txs in this block
	block.MerkleRoot = protocol.BuildMerkleTree(block).MerkleRoot()
	validatorAcc, err := storage.GetAccount(protocol.SerializeHashContent(validatorAccAddress))
	if err != nil {
		return err
	}

	validatorAccHash := validatorAcc.Hash()
	copy(block.Beneficiary[:], validatorAccHash[:])

	// Cryptographic Sortition for PoS in Bazo
	// The commitment proof stores a signed message of the Height that this block was created at.
	commitmentProof, err := crypto.SignMessageWithRSAKey(commPrivKey, fmt.Sprint(block.Height))
	if err != nil {
		return err
	}

	//Block hash with MerkleTree and therefore, including all transactions
	partialHash := block.HashBlock()

	//Block hash without MerkleTree and therefore, without any transactions
	partialHashWithoutMerkleRoot := block.HashBlockWithoutMerkleRoot()

	prevProofs := GetLatestProofs(activeParameters.numIncludedPrevProofs, block)
	nonce, err := proofOfStake(getDifficulty(), block.PrevHash, prevProofs, block.Height, validatorAcc.Balance, commitmentProof)
	if err != nil {
		//Delete all partially added transactions.
		if nonce == -2 {
			for _, tx := range storage.FundsTxBeforeAggregation {
				storage.WriteOpenTx(tx)
			}
			storage.DeleteAllFundsTxBeforeAggregation()
		}
		return err
	}

	var nonceBuf [8]byte
	binary.BigEndian.PutUint64(nonceBuf[:], uint64(nonce))
	block.Nonce = nonceBuf
	block.Timestamp = nonce

	//Put pieces together to get the final hash.
	block.Hash = sha3.Sum256(append(nonceBuf[:], partialHash[:]...))
	block.HashWithoutTx = sha3.Sum256(append(nonceBuf[:], partialHashWithoutMerkleRoot[:]...))

	//This doesn't need to be hashed, because we already have the merkle tree taking care of consistency.
	block.NrAccTx = uint16(len(block.AccTxData))
	block.NrFundsTx = uint16(len(block.FundsTxData))
	block.NrConfigTx = uint8(len(block.ConfigTxData))
	block.NrStakeTx = uint16(len(block.StakeTxData))
	block.NrAggTx = uint16(len(block.AggTxData))
	block.NrUpdateTx = uint16(len(block.UpdateTxData))

	copy(block.CommitmentProof[0:crypto.COMM_PROOF_LENGTH], commitmentProof[:])
	logger.Printf("-- End Finalization")

	storeBlockByTxs(block)

	return nil
}

//Transaction validation operates on a copy of a tiny subset of the state (all accounts involved in transactions).
//We do not operate global state because the work might get interrupted by receiving a block that needs validation
//which is done on the global state.
func addTx(b *protocol.Block, tx protocol.Transaction) error {
	//ActiveParameters is a datastructure that stores the current system parameters, gets only changed when
	//configTxs are broadcast in the network.

	//Switch this becasue aggtx fee is zero and otherwise this would lead to problems.
	switch tx.(type) {
	case *protocol.AggTx:
		return nil
	default:
		if tx.TxFee() < activeParameters.FeeMinimum {
			err := fmt.Sprintf("Transaction fee too low: %v (minimum is: %v)\n", tx.TxFee(), activeParameters.FeeMinimum)
			return errors.New(err)
		}
	}

	//There is a trade-off what tests can be made now and which have to be delayed (when dynamic state is needed
	//for inspection. The decision made is to check whether accTx and configTx have been signed with rootAcc. This
	//is a dynamic test because it needs to have access to the rootAcc state. The other option would be to include
	//the address (public key of signature) in the transaction inside the tx -> would resulted in bigger tx size.
	//So the trade-off is effectively clean abstraction vs. tx size. Everything related to fundsTx is postponed because
	//the txs depend on each other.
	if !verify(tx) {
		logger.Printf("Transaction could not be verified: %v", tx)
		return errors.New("Transaction could not be verified.")
	}

	switch tx.(type) {
	case *protocol.AccTx:
		err := addAccTx(b, tx.(*protocol.AccTx))
		if err != nil {
			logger.Printf("Adding accTx (%x) failed (%v): %v\n", tx.Hash(), err, tx.(*protocol.AccTx))

			return err
		}
	case *protocol.FundsTx:
		err := addFundsTx(b, tx.(*protocol.FundsTx))
		if err != nil {
			//logger.Printf("Adding fundsTx (%x) failed (%v): %v\n",tx.Hash(), err, tx.(*protocol.FundsTx))
			//logger.Printf("Adding fundsTx (%x) failed (%v)",tx.Hash(), err)
			return err
		}
	case *protocol.ConfigTx:
		err := addConfigTx(b, tx.(*protocol.ConfigTx))
		if err != nil {
			logger.Printf("Adding configTx (%x) failed (%v): %v\n", tx.Hash(), err, tx.(*protocol.ConfigTx))
			return err
		}
	case *protocol.StakeTx:
		err := addStakeTx(b, tx.(*protocol.StakeTx))
		if err != nil {
			logger.Printf("Adding stakeTx (%x) failed (%v): %v\n", tx.Hash(), err, tx.(*protocol.StakeTx))
			return err
		}
	case *protocol.UpdateTx:
		err := addUpdateTx(b, tx.(*protocol.UpdateTx))
		if err != nil {
			logger.Printf("Adding updateTx (%x) failed (%v): %v\n", tx.Hash(), err, tx.(*protocol.UpdateTx))
		}
	default:
		return errors.New("Transaction type not recognized.")
	}
	return nil
}

// Adds a mapping (key: txHash, value: blockHash) for all txs of a block to the local storage.
func storeBlockByTxs(block *protocol.Block) {

	// Agg
	for _, txHash := range block.AggTxData {
		storage.WriteBlockHashByTxHash(txHash, block.Hash)
	}

	// Funds
	for _, txHash := range block.FundsTxData {
		storage.WriteBlockHashByTxHash(txHash, block.Hash)
	}

	// Accounts
	for _, txHash := range block.AccTxData {
		storage.WriteBlockHashByTxHash(txHash, block.Hash)
	}

	// Config
	for _, txHash := range block.ConfigTxData {
		storage.WriteBlockHashByTxHash(txHash, block.Hash)
	}

	// Delete
	for _, txHash := range block.UpdateTxData {
		storage.WriteBlockHashByTxHash(txHash, block.Hash)
	}
}
