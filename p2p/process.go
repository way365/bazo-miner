package p2p

import (
	"encoding/binary"
	"github.com/way365/bazo-miner/protocol"
	"github.com/way365/bazo-miner/storage"
	"strconv"
	"sync"
)

var (
	processTxBroadcastMutex = &sync.Mutex{}
)

// Process tx broadcasts from other miners. We can't broadcast incoming messages directly, first check if
// the tx has already been broadcast before, whether it is a valid tx etc.
func processTxBrdcst(p *peer, payload []byte, brdcstType uint8) {

	var tx protocol.Transaction
	//Make sure the transaction can be properly decoded, verification is done at a later stage to reduce latency
	switch brdcstType {
	case FUNDSTX_BRDCST:
		var fTx *protocol.FundsTx
		fTx = fTx.Decode(payload)
		if fTx == nil {
			return
		}
		tx = fTx
	case ACCTX_BRDCST:
		var aTx *protocol.AccTx
		aTx = aTx.Decode(payload)
		if aTx == nil {
			return
		}
		tx = aTx
	case UPDATETX_BRDCST:
		var dTx *protocol.UpdateTx
		dTx = dTx.Decode(payload)
		if dTx == nil {
			return
		}
		tx = dTx
	case CONFIGTX_BRDCST:
		var cTx *protocol.ConfigTx
		cTx = cTx.Decode(payload)
		if cTx == nil {
			return
		}
		tx = cTx
	case STAKETX_BRDCST:
		var sTx *protocol.StakeTx
		sTx = sTx.Decode(payload)
		if sTx == nil {
			return
		}
		tx = sTx
	case AGGTX_BRDCST:
		var aTx *protocol.AggTx
		aTx = aTx.Decode(payload)
		if aTx == nil {
			return
		}
		tx = aTx
	}

	//Response tx acknowledgment if the peer is a client
	//if !peers.minerConns[p] {
	if !peers.contains(p.getIPPort(), PEERTYPE_MINER) {
		packet := BuildPacket(TX_BRDCST_ACK, nil)
		sendData(p, packet)
	}

	// Tx already in mempool of open tx

	if storage.ReadOpenTx(tx.Hash()) != nil {
		return
	}

	// Tx already in mempool of closed tx
	if storage.ReadClosedTx(tx.Hash()) != nil {
		return
	}

	logger.Printf("Received Tx: %x", tx.Hash())

	// If it's the first time, we receive the tx, we write to mempool and rebroadcast
	storage.WriteOpenTx(tx)
	toBrdcst := BuildPacket(brdcstType, payload)
	minerBrdcstMsg <- toBrdcst
}

func processTimeRes(p *peer, payload []byte) {
	time := int64(binary.BigEndian.Uint64(payload))
	//Concurrent writes need to be protected.
	//We use the same peer lock to prevent concurrent writes (on the network). It would be more efficient to use
	//different locks but the speedup is so marginal that it's not worth it.

	p.l.Lock()
	defer p.l.Unlock()
	p.time = time
}

func processNeighborRes(p *peer, payload []byte) {
	//Parse the incoming ipv4 addresses.
	ipportList := _processNeighborRes(payload)

	for _, ipportIter := range ipportList {
		//logger.Printf("IP/Port received: %v\n", ipportIter)
		//iplistChan is a buffered channel to handle ips asynchronously.
		if !peers.contains(ipportIter, PEERTYPE_MINER) && !peerSelfConn(ipportIter) && len(iplistChan) <= (MIN_MINERS*MIN_MINERS) {
			iplistChan <- ipportIter
		}
	}
}

// Split the processNeighborRes function in two for cleaner testing.
func _processNeighborRes(payload []byte) (ipportList []string) {
	index := 0

	for cnt := 0; cnt < len(payload)/(IPV4ADDR_SIZE+PORT_SIZE); cnt++ {
		var addr string
		for singleAddr := index; singleAddr < index+IPV4ADDR_SIZE; singleAddr++ {
			tmp := int(payload[singleAddr])
			addr += strconv.Itoa(tmp)
			addr += "."
		}
		//Remove trailing dot.
		addr = addr[:len(addr)-1]
		addr += ":"
		//Extract port number.
		addr += strconv.Itoa(int(binary.BigEndian.Uint16(payload[index+4 : index+6])))

		ipportList = append(ipportList, addr)
		index += IPV4ADDR_SIZE + PORT_SIZE
	}

	return ipportList
}

func EmptyingiplistChan() {
	for i := 0; i < len(iplistChan); i++ {
		<-iplistChan
	}
}
