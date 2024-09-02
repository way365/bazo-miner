package miner

import (
	"errors"
	"fmt"
	"github.com/way365/bazo-miner/p2p"
	"github.com/way365/bazo-miner/protocol"
	"github.com/way365/bazo-miner/storage"
	"time"
)

func addStakeTx(b *protocol.Block, tx *protocol.StakeTx) error {
	//Checking if the sender account is already in the local state copy. If not and account exist, create local copy
	//If account does not exist in state, abort.
	if _, exists := b.StateCopy[tx.Account]; !exists {
		if acc := storage.State[tx.Account]; acc != nil {
			hash := protocol.SerializeHashContent(acc.Address)
			if hash == tx.Account {
				newAcc := protocol.Account{}
				newAcc = *acc
				b.StateCopy[tx.Account] = &newAcc
			}
		} else {
			return errors.New(fmt.Sprintf("Sender account not present in the state: %x\n", tx.Account))
		}
	}

	//Root accounts are exempt from balance requirements. All other accounts need to have (at least)
	//fee + minimum amount that is required for staking.
	if !storage.IsRootKey(tx.Account) {
		if (tx.Fee + activeParameters.StakingMinimum) >= b.StateCopy[tx.Account].Balance {
			return errors.New("Not enough funds to complete the transaction!")
		}
	}

	//Account has bool already set to the desired value.
	if b.StateCopy[tx.Account].IsStaking == tx.IsStaking {
		return errors.New("Account has bool already set to the desired value.")
	}

	//Update state copy.
	accSender := b.StateCopy[tx.Account]
	accSender.IsStaking = tx.IsStaking
	accSender.CommitmentKey = tx.CommitmentKey

	//No further checks needed, static checks were already done with verify().
	b.StakeTxData = append(b.StakeTxData, tx.Hash())
	logger.Printf("Added tx (%x) to the StakeTxData slice: %v", tx.Hash(), *tx)
	return nil
}

func fetchStakeTxData(block *protocol.Block, stakeTxSlice []*protocol.StakeTx, initialSetup bool, errChan chan error) {
	for cnt, txHash := range block.StakeTxData {
		var tx protocol.Transaction
		var stakeTx *protocol.StakeTx

		closedTx := storage.ReadClosedTx(txHash)
		if closedTx != nil {
			if initialSetup {
				stakeTx = closedTx.(*protocol.StakeTx)
				stakeTxSlice[cnt] = stakeTx
				continue
			} else {
				errChan <- errors.New("Block validation had stakeTx that was already in a previous block.")
				return
			}
		}

		tx = storage.ReadOpenTx(txHash)
		if tx != nil {
			stakeTx = tx.(*protocol.StakeTx)
		} else {
			err := p2p.TxReq(txHash, p2p.STAKETX_REQ)
			if err != nil {
				errChan <- errors.New(fmt.Sprintf("StakeTx could not be read: %v", err))
				return
			}

			select {
			case stakeTx = <-p2p.StakeTxChan:
			case <-time.After(TXFETCH_TIMEOUT * time.Second):
				errChan <- errors.New("StakeTx fetch timed out.")
				return
			}
			if stakeTx.Hash() != txHash {
				errChan <- errors.New("Received StaketxHash did not correspond to our request.")
			}
		}

		stakeTxSlice[cnt] = stakeTx
	}

	errChan <- nil
}
