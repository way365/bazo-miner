package p2p

import (
	"github.com/way365/bazo-miner/storage"
	"log"
)

var (
	logger     *log.Logger
	LogMapping = map[uint8]string{
		1:  "FUNDSTX_BRDCST",
		2:  "ACCTX_BRDCST",
		3:  "CONFIGTX_BRDCST",
		4:  "STAKETX_BRDCST",
		5:  "VERIFIEDTX_BRDCST",
		6:  "BLOCK_BRDCST",
		7:  "BLOCK_HEADER_BRDCST",
		8:  "TX_BRDCST_ACK",
		9:  "AGGTX_BRDCST",
		10: "UPDATETX_BRDCST",

		20: "FUNDSTX_REQ",
		21: "ACCTX_REQ",
		22: "CONFIGTX_REQ",
		23: "STAKETX_REQ",
		24: "BLOCK_REQ",
		25: "BLOCK_HEADER_REQ",
		26: "ACC_REQ",
		27: "ROOTACC_REQ",
		28: "INTERMEDIATE_NODES_REQ",
		29: "AGGTX_REQ",
		30: "UNKNOWNTX_REQ",
		31: "SPECIALTX_REQ",
		32: "NOT_FOUND_TX_REQ",
		33: "UPDATETX_REQ",

		40: "FUNDSTX_RES",
		41: "ACCTX_RES",
		42: "CONFIGTX_RES",
		43: "STAKETX_RES",
		44: "BlOCK_RES",
		45: "BlOCK_HEADER_RES",
		46: "ACC_RES",
		47: "ROOTACC_RES",
		48: "INTERMEDIATE_NODES_RES",
		49: "AGGTX_RES",
		50: "UPDATETX_RES",

		130: "NEIGHBOR_REQ",
		140: "NEIGHBOR_RES",

		150: "TIME_BRDCST",

		100: "MINER_PING",
		101: "MINER_PONG",
		102: "CLIENT_PING",
		103: "CLIENT_PONG",

		110: "NOT_FOUND",
	}
)

func InitLogging() {
	logger = storage.InitLogger()
}
