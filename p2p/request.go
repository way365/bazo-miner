package p2p

//Requests that the p2p package issues requesting data from other miners.
//Tx and block requests are processed in the miner_interface.go file, because
//this involves inter-communication between the two packages
func neighborReq() {

//	p := peers.getRandomPeer(PEERTYPE_MINER)
//	if p == nil {
//		logger.Print("Could not fetch a random peer.\n")
//		return
//	}
	knownPeers := peers.minerConns
	for p := range knownPeers {
		packet := BuildPacket(NEIGHBOR_REQ, nil)
		sendData(p, packet)
	}

}

func neighborBrdcst() {

	//	p := peers.getRandomPeer(PEERTYPE_MINER)
	//	if p == nil {
	//		logger.Print("Could not fetch a random peer.\n")
	//		return
	//	}
	knownPeers := peers.minerConns
	var ipportList []string
	for p := range knownPeers {
		ipportList = append(ipportList, p.getIPPort())

	}

	packet := BuildPacket(NEIGHBOR_RES, _neighborRes(ipportList))
	for p := range knownPeers {
		sendData(p, packet)
	}

}
