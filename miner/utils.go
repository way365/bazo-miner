package miner

import (
	"github.com/way365/bazo-miner/protocol"
	"sort"
)

func InvertBlockArray(array []*protocol.Block) []*protocol.Block {
	for i, j := 0, len(array)-1; i < j; i, j = i+1, j-1 {
		array[i], array[j] = array[j], array[i]
	}
	return array
}

func getMaxKeyAndValueFormMap(m map[[32]byte]uint32) (uint32, [32]byte) {
	var max uint32 = 0
	biggestK := [32]byte{}
	for k := range m {
		if m[k] > max {
			max = m[k]
			biggestK = k
		}
	}

	return max, biggestK
}

// The next few functions below are used for sorting the List of transactions which can be aggregated.
// The Mempool is only sorted according to teh TxCount, So sorting the transactions which can be aggregated by sender
// and TxCount eases the aggregation process.
// It is implemented near to https://golang.org/pkg/sort/
type lessFunc func(p1, p2 *protocol.FundsTx) bool

type multiSorter struct {
	transactions []*protocol.FundsTx
	less         []lessFunc
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
