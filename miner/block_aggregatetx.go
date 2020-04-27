package miner

import (
	"errors"
	"fmt"
	"github.com/julwil/bazo-miner/p2p"
	"github.com/julwil/bazo-miner/protocol"
	"github.com/julwil/bazo-miner/storage"
	"sort"
	"time"
)

func AggregateTransactions(SortedAndSelectedFundsTx []protocol.Transaction, block *protocol.Block) error {
	aggregationMutex.Lock()
	defer aggregationMutex.Unlock()

	var transactionHashes, transactionReceivers, transactionSenders [][32]byte
	var nrOfSenders = map[[32]byte]int{}
	var nrOfReceivers = map[[32]byte]int{}
	var amount uint64
	var historicTransactions []protocol.Transaction

	//Sum up Amount, copy sender and receiver to correct slices and to map to check if aggregation by sender or receiver.
	for _, tx := range SortedAndSelectedFundsTx {

		trx := tx.(*protocol.FundsTx)
		amount += trx.Amount
		transactionSenders = append(transactionSenders, trx.From)
		nrOfSenders[trx.From] = nrOfSenders[trx.From] + 1
		transactionReceivers = append(transactionReceivers, trx.To)
		nrOfReceivers[trx.To] = nrOfReceivers[trx.To] + 1
		transactionHashes = append(transactionHashes, trx.Hash())
		storage.WriteOpenTx(trx)

	}

	//Check if only one sender or receiver is in the slices and if yes, shorten them.
	if len(nrOfSenders) == 1 {
		transactionSenders = transactionSenders[:1]
	}
	if len(nrOfReceivers) == 1 {
		transactionReceivers = transactionReceivers[:1]
	}

	// set addresses by which historic transactions should be searched.
	// If Zero-32byte array is sent, it will not find any addresses, because BAZOAccountAddresses
	if len(nrOfSenders) == len(nrOfReceivers) {
		//Here transactions are searched to aggregate. if one is fund, it will aggregate accordingly.
		breakingForLoop := false
		for _, block := range storage.ReadAllClosedBlocksWithTransactions() {
			//Search All fundsTx to check if there are transactions with the same sender orrReceiver
			for _, fundsTxHash := range block.FundsTxData {
				trx := storage.ReadClosedTx(fundsTxHash)
				if len(transactionSenders) > 0 && trx.(*protocol.FundsTx).From == transactionSenders[0] {
					historicTransactions = searchTransactionsInHistoricBlocks(transactionSenders[0], [32]byte{})
					breakingForLoop = true
					break
				} else if len(transactionReceivers) > 0 && trx.(*protocol.FundsTx).To == transactionReceivers[0] {
					historicTransactions = searchTransactionsInHistoricBlocks([32]byte{}, transactionReceivers[0])
					breakingForLoop = true
					break
				}
			}
			if breakingForLoop == true {
				break
			}
			//Search all aggTx
			for _, aggTxHash := range block.AggTxData {
				trx := storage.ReadClosedTx(aggTxHash)
				if trx != nil {
					if len(trx.(*protocol.AggTx).From) == 1 && len(transactionSenders) > 0 && trx.(*protocol.AggTx).From[0] == transactionSenders[0] {
						historicTransactions = searchTransactionsInHistoricBlocks(transactionSenders[0], [32]byte{})
						breakingForLoop = true
						break
					} else if len(trx.(*protocol.AggTx).To) == 1 && len(transactionReceivers) > 0 && trx.(*protocol.AggTx).To[0] == transactionReceivers[0] {
						historicTransactions = searchTransactionsInHistoricBlocks([32]byte{}, transactionReceivers[0])
						breakingForLoop = true
						break
					}
				}
			}
			if breakingForLoop == true {
				break
			}
		}
	} else if len(nrOfSenders) < len(nrOfReceivers) {
		historicTransactions = searchTransactionsInHistoricBlocks(transactionSenders[0], [32]byte{})
	} else if len(nrOfSenders) > len(nrOfReceivers) {
		historicTransactions = searchTransactionsInHistoricBlocks([32]byte{}, transactionReceivers[0])
	}

	//Add transactions to the transactionsHashes slice
	for _, tx := range historicTransactions {
		transactionHashes = append(transactionHashes, tx.Hash())
	}

	if len(transactionHashes) > 1 {

		//Create Aggregated Transaction
		aggTx, err := protocol.ConstrAggTx(
			amount,
			0,
			transactionSenders,
			transactionReceivers,
			transactionHashes,
		)

		if err != nil {
			logger.Printf("%v\n", err)
			return err
		}

		//Print aggregated Transaction
		logger.Printf("%v", aggTx)

		//Add Aggregated transaction and write to open storage
		addAggTxFinal(block, aggTx)
		storage.WriteOpenTx(aggTx)

		//Sett all back to "zero"
		SortedAndSelectedFundsTx = nil
		amount = 0
		transactionReceivers = nil
		transactionHashes = nil

	} else if len(SortedAndSelectedFundsTx) > 0 {
		addFundsTxFinal(block, SortedAndSelectedFundsTx[0].(*protocol.FundsTx))
	} else {
		err := errors.New("NullPointer")
		return err
	}

	return nil
}

func addAggTxFinal(b *protocol.Block, tx *protocol.AggTx) error {
	b.AggTxData = append(b.AggTxData, tx.Hash())
	sort.Sort(ByHash(b.AggTxData))
	return nil
}

func sortTxBeforeAggregation(Slice []*protocol.FundsTx) {
	//These Functions are inserted in the OrderBy function above. According to them the slice will be sorted.
	// It is sorted according to the Sender and the transactioncount.
	sender := func(c1, c2 *protocol.FundsTx) bool {
		return string(c1.From[:32]) < string(c2.From[:32])
	}
	txcount := func(c1, c2 *protocol.FundsTx) bool {
		return c1.TxCnt < c2.TxCnt
	}

	OrderedBy(sender, txcount).Sort(Slice)
}

func splitSortedAggregatableTransactions(b *protocol.Block) {
	txToAggregate := make([]protocol.Transaction, 0)
	moreTransactionsToAggregate := true

	PossibleTransactionsToAggregate := storage.ReadFundsTxBeforeAggregation()

	sortTxBeforeAggregation(PossibleTransactionsToAggregate)
	cnt := 0

	for moreTransactionsToAggregate {

		//Get Sender and Receiver which are most common
		maxSender, addressSender := getMaxKeyAndValueFormMap(storage.DifferentSenders)
		maxReceiver, addressReceiver := getMaxKeyAndValueFormMap(storage.DifferentReceivers)

		// The sender or receiver which is most common is selected and all transactions are added to the txToAggregate
		// slice. The number of transactions sent/Received will lower with every tx added. Then the splitted transactions
		// get aggregated into the correct aggregation transaction type and then written into the block.
		i := 0
		if maxSender >= maxReceiver {
			for _, tx := range PossibleTransactionsToAggregate {
				if tx.From == addressSender {
					txToAggregate = append(txToAggregate, tx)
				} else {
					PossibleTransactionsToAggregate[i] = tx
					i++
				}
			}
		} else {
			for _, tx := range PossibleTransactionsToAggregate {
				if tx.To == addressReceiver {
					txToAggregate = append(txToAggregate, tx)
				} else {
					PossibleTransactionsToAggregate[i] = tx
					i++
				}
			}

		}

		PossibleTransactionsToAggregate = PossibleTransactionsToAggregate[:i]
		storage.DifferentSenders = map[[32]byte]uint32{}
		storage.DifferentReceivers = map[[32]byte]uint32{}

		//Count senders and receivers again, because some transactions are removed now.
		for _, tx := range PossibleTransactionsToAggregate {
			storage.DifferentSenders[tx.From] = storage.DifferentSenders[tx.From] + 1
			storage.DifferentReceivers[tx.To] = storage.DifferentReceivers[tx.To] + 1
		}

		//Aggregate Transactions
		AggregateTransactions(txToAggregate, b)

		//Empty Slice
		txToAggregate = txToAggregate[:0]

		if len(PossibleTransactionsToAggregate) > 0 && len(storage.DifferentSenders) > 0 && len(storage.DifferentReceivers) > 0 {
			if cnt > 20 {
				moreTransactionsToAggregate = false
			}
			cnt++

			moreTransactionsToAggregate = true
		} else {
			cnt = 0
			moreTransactionsToAggregate = false
		}
	}

	storage.DeleteAllFundsTxBeforeAggregation()
}

func searchTransactionsInHistoricBlocks(searchAddressSender [32]byte, searchAddressReceiver [32]byte) (historicTransactions []protocol.Transaction) {

	for _, block := range storage.ReadAllClosedBlocksWithTransactions() {

		//Read all FundsTxIncluded in the block
		for _, txHash := range block.FundsTxData {
			tx := storage.ReadClosedTx(txHash)
			if tx != nil {
				trx := tx.(*protocol.FundsTx)
				if trx != nil && trx.Aggregated == false && (trx.From == searchAddressSender || trx.To == searchAddressReceiver) {
					historicTransactions = append(historicTransactions, trx)
					trx.Aggregated = true
					storage.WriteClosedTx(trx)
				}
			} else {
				return nil
			}
		}
		for _, txHash := range block.AggTxData {
			tx := storage.ReadClosedTx(txHash)
			if tx != nil {
				trx := tx.(*protocol.AggTx)
				if trx != nil && trx.Aggregated == false && (trx.From[0] == searchAddressSender || trx.To[0] == searchAddressReceiver) {
					logger.Printf("Found AggTx (%x) in (%x) which can be aggregated now.", trx.Hash(), block.Hash[0:8])
					historicTransactions = append(historicTransactions, trx)
					trx.Aggregated = true
					storage.WriteClosedTx(trx)
				}
			} else {
				//Tx Was not closes
				return nil
			}
		}
	}
	return historicTransactions
}

//This function fetches the AggTx's from a block. Furthermore it fetches missing transactions aggregated by these AggTx's
func fetchAggTxData(block *protocol.Block, aggTxSlice []*protocol.AggTx, initialSetup bool, errChan chan error, aggregatedFundsChan chan []*protocol.FundsTx) {
	var transactions []*protocol.FundsTx

	//First the aggTx is needed. Then the transactions aggregated in the AggTx
	for cnt, aggTxHash := range block.AggTxData {
		//Search transaction in closed transactions.
		aggTx := storage.ReadClosedTx(aggTxHash)

		//Check if transaction was already in another block, expect it is aggregated.
		if !initialSetup && aggTx != nil && aggTx.(*protocol.AggTx).Aggregated == false {
			if !aggTx.(*protocol.AggTx).Aggregated {
				logger.Printf("Block validation had AggTx (%x) that was already in a previous block.", aggTx.Hash())
				errChan <- errors.New("Block validation had AggTx that was already in a previous block.")
				return
			}
		}

		if aggTx == nil {
			//Read invalid storage when not found in closed & open Transactions
			aggTx = storage.ReadOpenTx(aggTxHash)
		}

		if aggTx == nil {
			//Read open storage when not found in closedTransactions
			aggTx = storage.ReadINVALIDOpenTx(aggTxHash)
		}
		if aggTx == nil {
			//Transaction need to be fetched from the network.
			cnt := 0
		HERE:
			logger.Printf("Request AGGTX: %x for block %x", aggTxHash, block.Hash[0:8])
			err := p2p.TxReq(aggTxHash, p2p.AGGTX_REQ)
			if err != nil {
				errChan <- errors.New(fmt.Sprintf("AggTx could not be read: %v", err))
				return
			}

			select {
			case aggTx = <-p2p.AggTxChan:
				storage.WriteOpenTx(aggTx)
				logger.Printf("  Received AGGTX: %x for block %x", aggTxHash, block.Hash[0:8])
			case <-time.After(TXFETCH_TIMEOUT * time.Second):
				stash := p2p.ReceivedAggTxStash
				if p2p.AggTxAlreadyInStash(stash, aggTxHash) {
					for _, tx := range stash {
						if tx.Hash() == aggTxHash {
							aggTx = tx
							logger.Printf("  FOUND: Request AGGTX: %x for block %x in received Stash during timeout", aggTx.Hash(), block.Hash[0:8])
							break
						}
					}
					break
				}
				if cnt < 2 {
					cnt++
					goto HERE
				}
				logger.Printf("  TIME OUT: Request AGGTX: %x for block %x", aggTxHash, block.Hash[0:8])
				errChan <- errors.New("AggTx fetch timed out")
				return
			}
			if aggTx.Hash() != aggTxHash {
				errChan <- errors.New("Received AggTxHash did not correspond to our request.")
			}
			logger.Printf("Received requested AggTX %x", aggTx.Hash())
		}

		//At this point the aggTx visible in the blocks body should be received.
		if aggTx == nil {
			errChan <- errors.New("AggTx Could not be found ")
			return
		}

		//Add Transaction to the aggTxSlice of the block.
		aggTxSlice[cnt] = aggTx.(*protocol.AggTx)

		//Now all transactions aggregated in the specific aggTx are handled. This are either new FundsTx which are
		//needed for the state update or other aggTx again aggregated. The later ones are validated in an older block.
		if initialSetup {
			//If this AggTx is Aggregated, all funds will be fetched later... --> take teh next AggTx.
			if aggTx.(*protocol.AggTx).Aggregated == true {
				continue
			}

			//All FundsTransactions are needed. Fetch them recursively. If an error occurs --> return.
			var err error
			transactions, err = fetchFundsTxRecursively(aggTx.(*protocol.AggTx).AggregatedTxSlice)
			if err != nil {
				errChan <- err
				return
			}
		} else {
			//Not all funds transactions are needed. Only the new ones. The other ones are already validated in the state.
			for _, txHash := range aggTx.(*protocol.AggTx).AggregatedTxSlice {
				tx := storage.ReadClosedTx(txHash)

				if tx != nil {
					//Found already closed transaction --> Not needed for further process.
					logger.Printf("Found Transaction %x which was in previous block.", tx.Hash())
					continue
				} else {
					tx = storage.ReadOpenTx(txHash)

					if tx != nil {
						//Found Open new Agg Transaction
						switch tx.(type) {
						case *protocol.AggTx:
							continue
						}
						transactions = append(transactions, tx.(*protocol.FundsTx))
					} else {
						if tx != nil {
							tx = storage.ReadINVALIDOpenTx(txHash)
							switch tx.(type) {
							case *protocol.AggTx:
								continue
							}
							transactions = append(transactions, tx.(*protocol.FundsTx))
						} else {
							//Need to fetch transaction from the network. At this point it is unknown what type of tx we request.
							cnt := 0
						NEXTTRY:
							err := p2p.TxReq(txHash, p2p.UNKNOWNTX_REQ)
							if err != nil {
								errChan <- errors.New(fmt.Sprintf("Tx could not be read: %v", err))
								return
							}

							//Depending on which channel the transaction is received, the type of the transaction is known.
							select {
							case tx = <-p2p.AggTxChan:
								//Received an Aggregated Transaction which was already validated in an older block.
								storage.WriteOpenTx(tx)
							case tx = <-p2p.FundsTxChan:
								//Received a fundsTransaction, which needs to be handled further.
								storage.WriteOpenTx(tx)
								transactions = append(transactions, tx.(*protocol.FundsTx))
							case <-time.After(TXFETCH_TIMEOUT * time.Second):
								aggTxStash := p2p.ReceivedAggTxStash
								if p2p.AggTxAlreadyInStash(aggTxStash, txHash) {
									for _, trx := range aggTxStash {
										if trx.Hash() == txHash {
											tx = trx
											storage.WriteOpenTx(tx)
											break
										}
									}
									break
								}
								fundsTxStash := p2p.ReceivedFundsTXStash
								if p2p.FundsTxAlreadyInStash(fundsTxStash, txHash) {
									for _, trx := range fundsTxStash {
										if trx.Hash() == txHash {
											tx = trx
											storage.WriteOpenTx(tx)
											transactions = append(transactions, tx.(*protocol.FundsTx))
											break
										}
									}
									break
								}
								if cnt < 2 {
									cnt++
									goto NEXTTRY
								}
								logger.Printf("Fetching UnknownTX %x timed out for block %x", txHash, block.Hash[0:8])
								errChan <- errors.New("UnknownTx fetch timed out")
								return
							}
							if tx.Hash() != txHash {
								errChan <- errors.New("Received TxHash did not correspond to our request.")
							}
						}
					}
				}
			}
		}
	}

	//Send the transactions into the channel, otherwise send nil such that a deadlock cannot occur.
	if len(transactions) > 0 {
		aggregatedFundsChan <- transactions
	} else {
		aggregatedFundsChan <- nil
	}

	errChan <- nil
}
