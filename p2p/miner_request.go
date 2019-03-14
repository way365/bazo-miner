package p2p

import (
	"errors"
)

//Both block and tx requests are handled asymmetricaly, using channels as inter-communication
//All the request in this file are specifically initiated by the miner package
//func BlockReq(hash [32]byte, hashWithoutTx [32]byte) error {
func BlockReq(hash [32]byte, hashWithoutTx [32]byte) error {

	logger.Printf("Request Block %x, %x from the network", hash[0:8], hashWithoutTx[0:8])

	payload := hash[:]
	payloadTEMP := hashWithoutTx[:]

	payload = append(payload, payloadTEMP...)

	// Block Request with a Broadcast request. This does rise the possibility of a valid answer.
	for p := range peers.minerConns {
		//Write to the channel, which the peerBroadcast(*peer) running in a seperate goroutine consumes right away.

		if p == nil {
			return errors.New("Couldn't get a connection, request not transmitted.")
		}
		packet := BuildPacket(BLOCK_REQ, payload)
		sendData(p, packet)
	}

	return nil
}

func LastBlockReq() error {

	p := peers.getRandomPeer(PEERTYPE_MINER)
	if p == nil {
		return errors.New("Couldn't get a connection, request not transmitted.")
	}

	packet := BuildPacket(BLOCK_REQ, nil)
	sendData(p, packet)
	return nil
}

//Request specific transaction
func TxReq(hash [32]byte, reqType uint8) error {

	// Tx Request also as brodcast so that the possibility of an answer is higher.
	for p := range peers.minerConns {
		//Write to the channel, which the peerBroadcast(*peer) running in a seperate goroutine consumes right away.

		if p == nil {
			return errors.New("Couldn't get a connection, request not transmitted.")
		}
		packet := BuildPacket(reqType, hash[:])
		sendData(p, packet)
	}

	return nil
}

//Request specific transaction
func TxWithTxCntReq(payload []byte, reqType uint8) error { //TODO

	// Tx Request also as brodcast so that the possibility of an answer is higher.
	for p := range peers.minerConns {
		//Write to the channel, which the peerBroadcast(*peer) running in a seperate goroutine consumes right away.

		if p == nil {
			return errors.New("Couldn't get a connection, request not transmitted.")
		}

		packet := BuildPacket(reqType, payload)
		sendData(p, packet)
	}

	return nil
}

func PrintMinerCons() {
	logger.Printf(" ____________")
	logger.Printf("| Neighbors: |______________")
	for p := range peers.minerConns {
		logger.Printf("|-- Miner: %v   |", p.getIPPort())
	}
	logger.Printf("|___________________________|")
}
