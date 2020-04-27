package miner

import (
	"errors"
	"fmt"
	"github.com/julwil/bazo-miner/p2p"
	"github.com/julwil/bazo-miner/protocol"
	"github.com/julwil/bazo-miner/storage"
	"github.com/julwil/bazo-miner/vm"
	"time"
)

func addFundsTx(b *protocol.Block, tx *protocol.FundsTx) error {

	addFundsTxMutex.Lock()

	//Checking if the sender account is already in the local state copy. If not and account exist, create local copy.
	//If account does not exist in state, abort.
	if _, exists := b.StateCopy[tx.From]; !exists {
		if acc := storage.State[tx.From]; acc != nil {
			hash := protocol.SerializeHashContent(acc.Address)
			if hash == tx.From {
				newAcc := protocol.Account{}
				newAcc = *acc
				b.StateCopy[tx.From] = &newAcc
			}
		} else {
			storage.WriteINVALIDOpenTx(tx)
			addFundsTxMutex.Unlock()
			return errors.New(fmt.Sprintf("Sender account not present in the state: %x\n", tx.From))
		}
	}

	//Vice versa for receiver account.
	if _, exists := b.StateCopy[tx.To]; !exists {
		if acc := storage.State[tx.To]; acc != nil {
			hash := protocol.SerializeHashContent(acc.Address)
			if hash == tx.To {
				newAcc := protocol.Account{}
				newAcc = *acc
				b.StateCopy[tx.To] = &newAcc
			}
		} else {
			storage.WriteINVALIDOpenTx(tx)
			addFundsTxMutex.Unlock()
			return errors.New(fmt.Sprintf("Receiver account not present in the state: %x\n", tx.To))
		}
	}

	//Root accounts are exempt from balance requirements. All other accounts need to have (at least)
	//fee + amount to spend as balance available.
	if !storage.IsRootKey(tx.From) {
		if (tx.Amount + tx.Fee) > b.StateCopy[tx.From].Balance {
			storage.WriteINVALIDOpenTx(tx)
			addFundsTxMutex.Unlock()
			return errors.New("Not enough funds to complete the transaction!")
		}
	}

	//Transaction count need to match the state, preventing replay attacks.
	if b.StateCopy[tx.From].TxCnt != tx.TxCnt {
		if tx.TxCnt < b.StateCopy[tx.From].TxCnt {
			closedTx := storage.ReadClosedTx(tx.Hash())
			if closedTx != nil {
				storage.DeleteOpenTx(tx)
				storage.DeleteINVALIDOpenTx(tx)
				addFundsTxMutex.Unlock()
				return nil
			} else {
				addFundsTxMutex.Unlock()
				return nil
			}
		} else {
			storage.WriteINVALIDOpenTx(tx)
		}
		err := fmt.Sprintf("Sender %x txCnt does not match: %v (tx.txCnt) vs. %v (state txCnt)\nAggrgated: %t", tx.From, tx.TxCnt, b.StateCopy[tx.From].TxCnt, tx.Aggregated)
		storage.WriteINVALIDOpenTx(tx)
		addFundsTxMutex.Unlock()
		return errors.New(err)
	}

	//Prevent balance overflow in receiver account.
	if b.StateCopy[tx.To].Balance+tx.Amount > MAX_MONEY {
		err := fmt.Sprintf("Transaction amount (%v) leads to overflow at receiver account balance (%v).\n", tx.Amount, b.StateCopy[tx.To].Balance)
		storage.WriteINVALIDOpenTx(tx)
		addFundsTxMutex.Unlock()
		return errors.New(err)
	}

	//Check if transaction has data and the receiver account has a smart contract
	if tx.Data != nil && b.StateCopy[tx.To].Contract != nil {
		context := protocol.NewContext(*b.StateCopy[tx.To], *tx)
		virtualMachine := vm.NewVM(context)

		// Check if vm execution run without error
		if !virtualMachine.Exec(false) {
			storage.WriteINVALIDOpenTx(tx)
			addFundsTxMutex.Unlock()
			return errors.New(virtualMachine.GetErrorMsg())
		}

		//Update changes vm has made to the contract variables
		context.PersistChanges()
	}

	//Update state copy.
	accSender := b.StateCopy[tx.From]
	accSender.TxCnt += 1
	accSender.Balance = accSender.Balance - (tx.Amount + tx.Fee)

	accReceiver := b.StateCopy[tx.To]
	accReceiver.Balance += tx.Amount

	//Add teh transaction to the storage where all Funds-transactions are stored before they where aggregated.
	storage.WriteFundsTxBeforeAggregation(tx)

	addFundsTxMutex.Unlock()
	return nil
}

func addFundsTxFinal(b *protocol.Block, tx *protocol.FundsTx) error {
	b.FundsTxData = append(b.FundsTxData, tx.Hash())
	return nil
}

func fetchFundsTxData(block *protocol.Block, fundsTxSlice []*protocol.FundsTx, initialSetup bool, errChan chan error) {
	for cnt, txHash := range block.FundsTxData {
		var tx protocol.Transaction
		var fundsTx *protocol.FundsTx

		closedTx := storage.ReadClosedTx(txHash)
		if closedTx != nil {
			if initialSetup {
				fundsTx = closedTx.(*protocol.FundsTx)
				fundsTxSlice[cnt] = fundsTx
				continue
			} else {
				logger.Printf("Block validation had fundsTx (%x) that was already in a previous block.", closedTx.Hash())
				errChan <- errors.New("Block validation had fundsTx that was already in a previous block.")
				return
			}
		}

		//We check if the Transaction is in the invalidOpenTX stash. When it is in there, and it is valid now, we save
		//it into the fundsTX and continue like usual. This additional stash does lower the amount of network requests.
		tx = storage.ReadOpenTx(txHash)
		txINVALID := storage.ReadINVALIDOpenTx(txHash)
		if tx != nil {
			fundsTx = tx.(*protocol.FundsTx)
		} else if txINVALID != nil && verify(txINVALID) {
			fundsTx = txINVALID.(*protocol.FundsTx)
		} else {
			err := p2p.TxReq(txHash, p2p.FUNDSTX_REQ)
			if err != nil {
				errChan <- errors.New(fmt.Sprintf("FundsTx could not be read: %v", err))
				return
			}
			select {
			case fundsTx = <-p2p.FundsTxChan:
				storage.WriteOpenTx(fundsTx)
				if initialSetup {
					storage.WriteBootstrapTxReceived(fundsTx)
				}
			case <-time.After(TXFETCH_TIMEOUT * time.Second):
				stash := p2p.ReceivedFundsTXStash
				if p2p.FundsTxAlreadyInStash(stash, txHash) {
					for _, tx := range stash {
						if tx.Hash() == txHash {
							fundsTx = tx
							break
						}
					}
					break
				} else {
					errChan <- errors.New("FundsTx fetch timed out")
					return
				}
			}
			if fundsTx.Hash() != txHash {
				errChan <- errors.New("Received FundstxHash did not correspond to our request.")
			}
		}

		fundsTxSlice[cnt] = fundsTx
	}

	errChan <- nil
}

//This function fetches the funds transactions recursively --> When a aggTx is agregated in another aggTx.
// This is mainly needed for the startup process. It is recursively searching until only funds transactions are in the list.
func fetchFundsTxRecursively(AggregatedTxSlice [][32]byte) (aggregatedFundsTxSlice []*protocol.FundsTx, err error) {
	for _, txHash := range AggregatedTxSlice {
		//Try To read the transaction from closed storage.
		tx := storage.ReadClosedTx(txHash)

		if tx == nil {
			//Try to read it from open storage
			tx = storage.ReadOpenTx(txHash)
		}
		if tx == nil {
			//Read invalid storage when not found in closed & open Transactions
			tx = storage.ReadINVALIDOpenTx(txHash)
		}
		if tx == nil {
			//Fetch it from the network.
			err := p2p.TxReq(txHash, p2p.UNKNOWNTX_REQ)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("RECURSIVE Tx could not be read: %v", err))

			}

			//Depending on which channel the transaction is received, the type of the transaction is known.
			select {
			case tx = <-p2p.AggTxChan:
			case tx = <-p2p.FundsTxChan:
			case <-time.After(TXFETCH_TIMEOUT * time.Second):
				stash := p2p.ReceivedFundsTXStash
				aggTxStash := p2p.ReceivedAggTxStash

				if p2p.FundsTxAlreadyInStash(stash, txHash) {
					for _, trx := range stash {
						if trx.Hash() == txHash {
							tx = trx
							break
						}
					}
					break
				} else if p2p.AggTxAlreadyInStash(aggTxStash, txHash) {
					for _, trx := range stash {
						if trx.Hash() == txHash {
							tx = trx
							break
						}
					}
					break
				} else {
					logger.Printf("RECURSIVE Fetching (%x) timed out...", txHash)
					return nil, errors.New(fmt.Sprintf("RECURSIVE UnknownTx fetch timed out"))
				}
			}
			if tx.Hash() != txHash {
				return nil, errors.New(fmt.Sprintf("RECURSIVE Received TxHash did not correspond to our request."))
			}
			storage.WriteOpenTx(tx)
		}
		switch tx.(type) {
		case *protocol.FundsTx:
			aggregatedFundsTxSlice = append(aggregatedFundsTxSlice, tx.(*protocol.FundsTx))
		case *protocol.AggTx:
			//Do a recursive re-call for this function and append it to the Slice. Add temp just below
			temp, error := fetchFundsTxRecursively(tx.(*protocol.AggTx).AggregatedTxSlice)
			aggregatedFundsTxSlice = append(aggregatedFundsTxSlice, temp...)
			err = error

		}
	}

	if err == nil {
		return aggregatedFundsTxSlice, nil
	} else {
		return nil, err
	}

}
