package miner

import (
	"encoding/binary"
	"github.com/bazo-blockchain/bazo-miner/p2p"
	"github.com/bazo-blockchain/bazo-miner/protocol"
	"github.com/bazo-blockchain/bazo-miner/storage"
	"sort"
	"time"
)

//The code here is needed if a new block is built. All open (not yet validated) transactions are first fetched
//from the mempool and then sorted. The sorting is important because if transactions are fetched from the mempool
//they're received in random order (because it's implemented as a map). However, if a user wants to issue more fundsTxs
//they need to be sorted according to increasing txCnt, this greatly increases throughput.

type openTxs []protocol.Transaction

func prepareBlock(block *protocol.Block) {
	logger.Printf("~~~~~~ Start Prepare Block")
	//Fetch all txs from mempool (opentxs).
	opentxs := storage.ReadAllOpenTxs()
	opentxs = append(opentxs, storage.ReadAllINVALIDOpenTx()...)
	var opentxToAdd []protocol.Transaction

	//This copy is strange, but seems to be necessary to leverage the sort interface.
	//Shouldn't be too bad because no deep copy.
	var tmpCopy openTxs
	tmpCopy = opentxs
	sort.Sort(tmpCopy)

	nonAggregatableTxCounter := 0 //Counter for all transactions which will not be aggregated. (Stake-, config-, acctx)
	transactionCounter := 0 //Counter for all transactions which will be aggregated. (Funds-, AggTx)

	blockSize := block.GetSize()+block.GetBloomFilterSize()
	transactionHashSize := 32  //It is 32 bytes

	//map where all senders from FundsTx and AggTx are added to. --> this ensures that tx with same sender are only counted once.
	storage.DifferentSenders = map[[32]byte]uint32{}
	storage.DifferentReceivers = map[[32]byte]uint32{}
	storage.FundsTxBeforeAggregation = nil

	type senderTxCount struct {
		sender [32]byte
		txcnt uint32
		missingTransactions []uint32
	}

	var MinimalTxCntPerSender = map[[32]byte]*senderTxCount{}

	//Check how many transactions can be added.
	for _, tx := range opentxs {
		//Switch because with an if statement every transaction would need a getter-method for its type.
		//Therefore, switch is more code-efficient.
		switch tx.(type) {
		case *protocol.FundsTx:
			trx := tx.(*protocol.FundsTx)
			storage.DifferentSenders[trx.From] = storage.DifferentSenders[trx.From] + 1
			storage.DifferentReceivers[trx.To] = storage.DifferentReceivers[trx.To] + 1

			//Create Mininmal txCnt for the different senders with stateTxCnt.. This is used to fetch missing transactions later on.

			if MinimalTxCntPerSender[trx.From] == nil {
				logger.Printf("New MinimalTxCntPerSender for %x, txcnt: %v", trx.From, storage.State[trx.From].TxCnt-1)
				MinimalTxCntPerSender[trx.From] = &senderTxCount{trx.From,  storage.State[trx.From].TxCnt-1, nil}

				logger.Printf("- Intermediate State:\n%v", getState())
				logger.Printf("  - Intermediate Acc %x txcnt -> %v", trx.From, storage.State[trx.From].TxCnt)
				logger.Printf("  - Intermediate State per Acc %v", storage.State[trx.From])
				logger.Printf("  - Intermediate MinimalTxCnt %x txCnt: %v",  MinimalTxCntPerSender[trx.From].sender, MinimalTxCntPerSender[trx.From].txcnt)
			}

			logger.Printf("Appending to sender %x TxCnt xyz upto tx %v, minimalTxCntPerSedner: %v", trx.Sender(), trx.TxCnt, MinimalTxCntPerSender[trx.From].txcnt+1)



			for i := MinimalTxCntPerSender[trx.From].txcnt+1; i < trx.TxCnt; i++ {
				logger.Printf("Appending to sender %x TxCnt %v upto tx %x, minimalTxCntPerSedner: %v", trx.Sender(), i, trx.TxCnt, MinimalTxCntPerSender[trx.From].txcnt+1)
				MinimalTxCntPerSender[trx.From].missingTransactions = append(MinimalTxCntPerSender[trx.From].missingTransactions, i)
			}

			MinimalTxCntPerSender[trx.From].txcnt = trx.TxCnt

		case *protocol.AggTx:
			storage.DifferentSenders[tx.Sender()] = storage.DifferentSenders[tx.Sender()] + 1
			storage.DifferentReceivers[tx.Receiver()] = storage.DifferentReceivers[tx.Receiver()] + 1
		default:
			nonAggregatableTxCounter += 1
		}

		//The Maximum Number of transactions is always the smaller number of Different Senders or Different Receivers.
		if len(storage.DifferentSenders) <= len(storage.DifferentReceivers) {
			transactionCounter = len(storage.DifferentSenders)
		} else {
			transactionCounter = len(storage.DifferentReceivers)
		}

		//Check if block will become to big when adding the next transaction.
		if int(blockSize)+(transactionCounter+nonAggregatableTxCounter)*transactionHashSize > int(activeParameters.Block_size) {
			logger.Printf("Block Would Overflow --> Stop adding new Transactions")
			break
		} else {
			opentxToAdd = append(opentxToAdd, tx)
		}
	}



	//Special Request for transactions missing between the Tx with the lowest TxCnt and the state.
	// With this transactions may are validated quicker.
	logger.Printf(":::: Start Fetching Missing Transactions")
	for _, sender := range MinimalTxCntPerSender {
		//if sender.minTxcnt > storage.State[sender.sender].TxCnt {
		//	logger.Printf("Missing Transaction:  Sender: %x  TxCnt: From %v to %v", sender.sender, storage.State[sender.sender].TxCnt, sender.minTxcnt)

		logger.Printf("Missing Transaction:    All these Transactions are missing for sender %x: %v ",sender.sender, MinimalTxCntPerSender[sender.sender].missingTransactions)


		//Try Fetching all transactions which are missing.
			//for searchTxcnt:= storage.State[sender.sender].TxCnt; searchTxcnt <= (sender.maxTxcnt); searchTxcnt++ {
			for _, searchTxcnt := range sender.missingTransactions {
			//	found := false
				logger.Printf("Missing Transaction:    Try Finding tx with txcnt %v in the closedStorage ", searchTxcnt)
				var tx protocol.Transaction
				//Look for tx in openStorage
				for _, trx := range storage.ReadAllOpenTxs() {
					switch trx.(type) {
					case *protocol.FundsTx:
						if trx.(*protocol.FundsTx).TxCnt == searchTxcnt&& trx.(*protocol.FundsTx).From == sender.sender {
							tx = trx
							logger.Printf("Missing Transaction:      Found tx with txcnt %v in the openStorage now", searchTxcnt)
							break
						} else {
							continue
						}
					default:
						continue
					}
					break
				}

				//Look for tx In closedStorage
				if tx == nil {
					logger.Printf("Missing Transaction:    Try Finding tx with txcnt %v in the OpenInvalidStorage", searchTxcnt)
					for _, trx := range storage.ReadAllINVALIDOpenTx() {
						switch trx.(type) {
						case *protocol.FundsTx:
							if trx.(*protocol.FundsTx).TxCnt == searchTxcnt && trx.(*protocol.FundsTx).From == sender.sender {
								tx = trx
								logger.Printf("Missing Transaction:      Found tx with txcnt %v in the OpenInvalidStorage now", searchTxcnt)
								break
							} else {
								continue
							}
						default:
							continue
						}
						break
					}
				}

				//Search in the open InvalidStorage
				if tx == nil {
					logger.Printf("Missing Transaction:    Try Finding tx with txcnt %v in the closedStorage ", searchTxcnt)
					for _, trx := range storage.ReadAllClosedFundsAndAggTransactions() {
						switch trx.(type) {
						case *protocol.FundsTx:
							if trx.(*protocol.FundsTx).TxCnt == searchTxcnt && trx.(*protocol.FundsTx).From == sender.sender {
								tx = trx
								logger.Printf("Missing Transaction:      Found tx with txcnt %v in the ClosedStorage now", searchTxcnt)
								break
							} else {
								continue
							}
						default:
							continue
						}
						break
					}
				}

				//Try to fetch tx from the network.
				if tx == nil {
					var requestTx = specialTxRequest{sender.sender, p2p.SPECIALTX_REQ, searchTxcnt}

					payload := requestTx.Encoding()
					//Special Request can be received through the fundsTxChan.
					logger.Printf("Missing Transaction:      Try Fetching tx with txcnt %v from the network now", searchTxcnt)
					err := p2p.TxWithTxCntReq(payload, p2p.SPECIALTX_REQ)
					if err != nil {
						continue
					}
					select {
					case trx := <-p2p.FundsTxChan:
						if trx.TxCnt != searchTxcnt&& trx.From != sender.sender {
							logger.Printf("Missing Transaction:         Received Wrong Transaction")
							break
						} else {
							logger.Printf("Missing Transaction:         Received Correct Tx through TxCnt Request: %x with txcnt %v", trx.Hash(), trx.TxCnt)
							storage.WriteOpenTx(trx, 16)
							tx = trx
							break
						}
					case <-time.After(TXFETCH_TIMEOUT * time.Second):
						logger.Printf("Missing Transaction:         Special Tx Request Timed out...")
						break
					}
				}

				if tx == nil {
					logger.Printf("Missing Transaction:         Not Reveiced the missing transaction with TxCnt: %v", searchTxcnt)
					continue
				}

				logger.Printf("Missing Transaction:      Append tx (%x), From %x  with txcnt %v now", tx.Hash(), tx.Sender(), tx.(*protocol.FundsTx).TxCnt)
				opentxToAdd = append(opentxToAdd, tx)
				tx = nil
			}
		//}
	}

	MinimalTxCntPerSender = nil
	logger.Printf(":::: End Fetching Missing Transactions")
	//Sort Tx Again to get lowest TxCnt at the beginning.
	tmpCopy = opentxToAdd
	sort.Sort(tmpCopy)

	//for _, tx := range opentxToAdd {
//
	//	switch tx.(type) {
	//	case *protocol.FundsTx:
	//		logger.Printf("Tx. %x, txcnt %v", tx.Hash(), tx.(*protocol.FundsTx).TxCnt)
	//	}
	//}

	//Add previous selected transactions.
	logger.Printf("::::  Start Adding Tx")
	for _, tx := range opentxToAdd {
		err := addTx(block, tx)
		if err != nil {
			//If the tx is invalid, we remove it completely, prevents starvation in the mempool.
			storage.DeleteOpenTx(tx, 1001)
		}
	}
	logger.Printf("::::  End Adding Tx")

	// In miner\block.go --> AddFundsTx the transactions get added into storage.TxBeforeAggregation.
	if len(storage.ReadFundsTxBeforeAggregation()) > 0 {
		splitSortedAggregatableTransactions(block)
	}

	//Set measurement values back to zero / nil.
	storage.DifferentSenders = nil
	storage.DifferentReceivers = nil
	nonAggregatableTxCounter = 0
	logger.Printf("~~~~~~ End Prepare Block")
	return
}

type specialTxRequest struct {
	senderHash [32]byte
	reqType    uint8
	txcnt      uint32
}

func (R *specialTxRequest) Encoding() (encodedTx []byte) {

	// Encode
	if R == nil {
		return nil
	}
	var txcnt [8]byte
	binary.BigEndian.PutUint32(txcnt[:], R.txcnt)
	encodedTx = make([]byte, 42)

	encodedTx[0] = R.reqType
	copy(encodedTx[1:9], txcnt[:])
	copy(encodedTx[10:42], R.senderHash[:])

	return encodedTx
}

//Implement the sort interface
func (f openTxs) Len() int {
	return len(f)
}

func (f openTxs) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func (f openTxs) Less(i, j int) bool {
	//Comparison only makes sense if both tx are fundsTxs.
	//Why can we only do that with switch, and not e.g., if tx.(type) == ..?
	switch f[i].(type) {
	case *protocol.AccTx:
		//We only want to sort a subset of all transactions, namely all fundsTxs.
		//However, to successfully do that we have to place all other txs at the beginning.
		//The order between accTxs and configTxs doesn't matter.
		return true
	case *protocol.ConfigTx:
		return true
	case *protocol.StakeTx:
		return true
	case *protocol.AggTx:
		return true
	}

	switch f[j].(type) {
	case *protocol.AccTx:
		return false
	case *protocol.ConfigTx:
		return false
	case *protocol.StakeTx:
		return false
	case *protocol.AggTx:
		return false
	}

	return f[i].(*protocol.FundsTx).TxCnt < f[j].(*protocol.FundsTx).TxCnt
}