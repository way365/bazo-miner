package p2p

import (
	"github.com/bazo-blockchain/bazo-miner/protocol"
	"github.com/bazo-blockchain/bazo-miner/storage"
	"time"
)


var (
	messages    [][]byte
	sendingMap  map[*peer][][]byte
)

//This is not accessed concurrently, one single goroutine. However, the "peers" are accessed concurrently, therefore the
//Thread-safe implementation.
func peerService() {
	for {
		select {
		case p := <-register:
			peers.add(p)
		case p := <-disconnect:
			if peers.contains(p.getIPPort(), PEERTYPE_MINER){
				logger.Printf("CHANNEL: Closed channel to %v", p.getIPPort()) //TODO: remove this
			}
			peers.delete(p)
			close(p.ch)
			if peers.contains(p.getIPPort(), PEERTYPE_MINER){
				logger.Printf("CHANNEL: Closed channel to %v", p.getIPPort())
			}
		}
	}
}

func broadcastService() {
	sendingMap = make(map[*peer][][]byte)

	for {
		select {
		//Broadcasting all messages.
		case msg := <-minerBrdcstMsg:
			minerBrdcstMsgMutex.Unlock()
			for p := range peers.minerConns {
				if !isConnectionAlreadyInSendingMap(p, sendingMap) {
					sendingMap[p] = nil
				}
			}
			sendAndSearchMessages(msg)
		case msg := <-clientBrdcstMsg:
			for p := range peers.clientConns {
				if peers.contains(p.getIPPort(),PEERTYPE_CLIENT) {
					p.ch <- msg
				} else {
					logger.Printf("CHANNEL_CLIENT: Wanted to send to %v, but %v is not in the peers.minerConns anymore", p.getIPPort(), p.getIPPort())
				}
			}
		}
	}
}

//This function does send the current and possible previous not send messages
func sendAndSearchMessages(msg []byte) {
	for p := range sendingMap {
		//Check if there is a valid transaction to peer p, if not, store message
		if peers.minerConns[p] {

			if peers.contains(p.getIPPort(),PEERTYPE_MINER) {
				p.ch <- msg
			} else {
				logger.Printf("CHANNEL_MINER: Wanted to send to %v, but %v is not in the peers.minerConns anymore", p.getIPPort(), p.getIPPort())
			}


			switch msg[4] {
			case FUNDSTX_BRDCST:
				var fTx *protocol.FundsTx
				fTx = fTx.Decode(msg[5:])
				if fTx == nil {
					return
				}
			//	logger.Printf("    SEND FundsTx %x to %v", fTx.Hash(), p.getIPPort())
			case AGGTX_BRDCST:
				var aTx *protocol.AggTx
				aTx = aTx.Decode(msg[5:])
				if aTx == nil {
					return
				}
			//	logger.Printf("    SEND AggTx %x to %v", aTx.Hash(), p.getIPPort())
			}




			for _, hMsg := range sendingMap[p] {
				//Send historic not yet sent transaction and remove it.
				if peers.contains(p.getIPPort(),PEERTYPE_MINER) {
					p.ch <- hMsg
				} else {
					logger.Printf("CHANNEL_MINER: Wanted to send to %v, but %v is not in the peers.minerConns anymore", p.getIPPort(), p.getIPPort())
				}



				switch msg[4] {
					case FUNDSTX_BRDCST:
						var fTx *protocol.FundsTx
						fTx = fTx.Decode(hMsg[5:])
						if fTx == nil {
							return
						}
						//logger.Printf("    SEND Historic FundsTx %x to %v", fTx.Hash(), p.getIPPort())
					case AGGTX_BRDCST:
						var aTx *protocol.AggTx
						aTx = aTx.Decode(hMsg[5:])
						if aTx == nil {
							return
						}
						//logger.Printf("    SEND Historic AggTx %x to %v", aTx.Hash(), p.getIPPort())
				}





				sendingMap[p] = sendingMap[p][1:]
			}
		} else {
			messages = sendingMap[p]
			//Check that historic messages are not becomming ot big...
			if len(messages) > 2000 {
				messages = messages[1:]
			}
			//Store message for this specific miner connection.
			sendingMap[p] = append(messages, msg)
		}
	}
}

func isConnectionAlreadyInSendingMap(p *peer, sendingMap map[*peer][][]byte) bool {
	for con := range sendingMap {
		if con == p {
			return true
		}
	}
	return false
}

//Belongs to the broadcast service.
func peerBroadcast(p *peer) {
	for msg := range p.ch {
		sendData(p, msg)
	}
}

//Single goroutine that makes sure the system is well connected.
func checkHealthService() {
	for {
		//time.Sleep(HEALTH_CHECK_INTERVAL * time.Second)  //Normal searching for neighbours
		var nrOfMiners = 1
		if len(peers.minerConns) > 1 {
			nrOfMiners = len(peers.minerConns)
		}

		time.Sleep(time.Duration(nrOfMiners) * 5 * time.Second)  //Dynamic searching for neighbours interval --> 5 times the number of miners

		if Ipport != storage.Bootstrap_Server && !peers.contains(storage.Bootstrap_Server, PEERTYPE_MINER) {
			p, err := initiateNewMinerConnection(storage.Bootstrap_Server)
			if p == nil || err != nil {
				logger.Printf("%v\n", err)
			} else {
				go peerConn(p)
			}
		}

		//Periodically check if we are well-connected
		if len(peers.minerConns) >= MIN_MINERS {
			continue
		}

		//The only goto in the code (I promise), but best solution here IMHO.
	RETRY:
		select {
		//iplistChan gets filled with every incoming neighborRes, they're consumed here.
		case ipaddr := <-iplistChan:
			p, err := initiateNewMinerConnection(ipaddr)
			if err != nil {
				logger.Printf("%v\n", err)
			}
			if p == nil || err != nil {
				goto RETRY
			}
			go peerConn(p)
			break
		default:
			//In case we don't have any ip addresses in the channel left, make a request to the network.
			neighborReq()
			logger.Printf("Request Neighbors... ")
			break
		}
	}
}

//Calculates periodically system time from available sources and broadcasts the time to all connected peers.
func timeService() {
	//Initialize system time.
	systemTime = time.Now().Unix()
	go func() {
		for {
			time.Sleep(UPDATE_SYS_TIME * time.Second)
			writeSystemTime()
		}
	}()

	for {
		time.Sleep(TIME_BRDCST_INTERVAL * time.Second)
		packet := BuildPacket(TIME_BRDCST, getTime())
		minerBrdcstMsgMutex.Lock()
		minerBrdcstMsg <- packet
	}
}
