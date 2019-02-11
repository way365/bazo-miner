package miner

import (
	"github.com/bazo-blockchain/bazo-miner/protocol"
	"github.com/bazo-blockchain/bazo-miner/storage"
	"sort"
)

//The code here is needed if a new block is built. All open (not yet validated) transactions are first fetched
//from the mempool and then sorted. The sorting is important because if transactions are fetched from the mempool
//they're received in random order (because it's implemented as a map). However, if a user wants to issue more fundsTxs
//they need to be sorted according to increasing txCnt, this greatly increases throughput.

type openTxs []protocol.Transaction

func prepareBlock(block *protocol.Block) {
	//Fetch all txs from mempool (opentxs).
	opentxs := storage.ReadAllOpenTxs()

	//This copy is strange, but seems to be necessary to leverage the sort interface.
	//Shouldn't be too bad because no deep copy.
	var tmpCopy openTxs
	tmpCopy = opentxs
	sort.Sort(tmpCopy)

	logger.Printf( "Open Transactions to be validated: %v", len(opentxs))

	//Counter for all transactions which will not be aggregated. (Stake-, config-, acctx)
	nonAggregatableTxCounter := 0
	blockSize := block.GetSize()+block.GetBloomFilterSize()

	//map where all senders from FundsTx and AggTx are added to. --> this ensures that tx with same sender are only counted once.
	storage.DifferentSenders = map[[32]byte][32]byte{}
	for _, tx := range opentxs {

		//Switch because with an if statement every transaction would need a getter-method for its type.
		// Therefore, switch is more code efficient.
		switch tx.(type) {
		case *protocol.FundsTx, *protocol.AggTxSender:
			storage.DifferentSenders[tx.Sender()] = tx.Sender()
		default:
			nonAggregatableTxCounter += 1
		}

		//Check if block will become to big when adding the next transaction.
		if int(blockSize)+
			(len(storage.DifferentSenders)*int(len(tx.Hash()))) +
			(int(nonAggregatableTxCounter)*int(len(tx.Hash()))) > int(activeParameters.Block_size){
			break
		}

		err := addTx(block, tx)
		if err != nil {
			//If the tx is invalid, we remove it completely, prevents starvation in the mempool.
			storage.WriteINVALIDOpenTx(tx)
			storage.DeleteOpenTx(tx)
		}
	}

	// In miner\block.go --> AddFundsTx the transactions get added into storage.FundsTxBeforeAggregation.
	// This will be sorted below.
	sortFundsTxBeforeAggregation()

	//Set measurement values back to zero / nil.
	storage.DifferentSenders = nil
	nonAggregatableTxCounter = 0

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
	}

	switch f[j].(type) {
	case *protocol.AccTx:
		return false
	case *protocol.ConfigTx:
		return false
	case *protocol.StakeTx:
		return false
	}

	return f[i].(*protocol.FundsTx).TxCnt < f[j].(*protocol.FundsTx).TxCnt
}

// This functions below here are used for sorting the List of transactions which can be aggregated.
// The Mempool is only sorted according to teh TxCount, So sorting the transactions which can be aggregated by sender
// and TxCount eases the aggregation process.
// It is implemented near to https://golang.org/pkg/sort/

type lessFunc func(p1, p2 *protocol.FundsTx) bool

type multiSorter struct {
	transactions []*protocol.FundsTx
	less    []lessFunc
}

func (ms *multiSorter) Sort(transactionsToSort []*protocol.FundsTx) {
	ms.transactions = transactionsToSort
	sort.Sort(ms)
}

func OrderedBy(less ...lessFunc) *multiSorter {
	return &multiSorter{
		less: less,
	}
}

func (ms *multiSorter) Len() int {
	return len(ms.transactions)
}

func (ms *multiSorter) Swap(i, j int) {
	ms.transactions[i], ms.transactions[j] = ms.transactions[j], ms.transactions[i]
}

func (ms *multiSorter) Less(i, j int) bool {
	p, q := ms.transactions[i], ms.transactions[j]
	var k int
	for k = 0; k < len(ms.less)-1; k++ {
		less := ms.less[k]
		switch {
		case less(p, q):
			return true
		case less(q, p):
			return false
		}
	}
	return ms.less[k](p, q)
}

func sortFundsTxBeforeAggregation() {
	//These Functions are inserted in the OrderBy function above. According to them the slice will be sorted.
	sender := func(c1, c2 *protocol.FundsTx) bool {
		return string(c1.From[:32]) < string(c2.From[:32])
	}
	txcount:= func(c1, c2 *protocol.FundsTx) bool {
		return c1.TxCnt< c2.TxCnt
	}

	OrderedBy(sender, txcount).Sort(storage.FundsTxBeforeAggregation)
}
