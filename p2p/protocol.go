package p2p

import "fmt"

const HEADER_LEN = 5

//Mapping constants, used to parse incoming messages
const (
	FUNDSTX_BRDCST     		= 1
	ACCTX_BRDCST       		= 2
	CONFIGTX_BRDCST    		= 3
	STAKETX_BRDCST     		= 4
	VERIFIEDTX_BRDCST  		= 5
	BLOCK_BRDCST       		= 6
	BLOCK_HEADER_BRDCST		= 7
	TX_BRDCST_ACK      		= 8
	AGGSENDERTX_BRDCST      = 9
	AGGRECEIVERTX_BRDCST    = 10

	FUNDSTX_REQ            	= 20
	ACCTX_REQ              	= 21
	CONFIGTX_REQ           	= 22
	STAKETX_REQ            	= 23
	BLOCK_REQ              	= 24
	BLOCK_HEADER_REQ       	= 25
	ACC_REQ                	= 26
	ROOTACC_REQ            	= 27
	INTERMEDIATE_NODES_REQ 	= 28
	AGGSENDERTX_REQ			= 29 //FABIO Is new
	AGGRECEIVERTX_REQ		= 30 //FABIO Is new

	FUNDSTX_RES            	= 40
	ACCTX_RES              	= 41
	CONFIGTX_RES           	= 42
	STAKETX_RES            	= 43
	BLOCK_RES              	= 44
	BlOCK_HEADER_RES       	= 45
	ACC_RES                	= 46
	ROOTACC_RES            	= 47
	INTERMEDIATE_NODES_RES 	= 48
	AGGSENDERTX_RES			= 49 //FABIO Is new
	AGGRECEIVERTX_RES		= 50 //FABIO Is new

	NEIGHBOR_REQ = 130
	NEIGHBOR_RES = 140

	TIME_BRDCST = 150

	MINER_PING  = 100
	MINER_PONG  = 101
	CLIENT_PING = 102
	CLIENT_PONG = 103

	//Used to signal error
	NOT_FOUND = 110
)

type Header struct {
	Len    uint32
	TypeID uint8
}

func (header Header) String() string {
	return fmt.Sprintf(
		"Length: %v\n"+
			"TypeID: %v\n",
		header.Len,
		header.TypeID,
	)
}
