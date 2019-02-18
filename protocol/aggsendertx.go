package protocol

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

const (
	AGGSENDERTX_SIZE = 53 //Only constant Values --> Without To & AggregatedTxSlice
)

//when we broadcast transactions we need a way to distinguish with a type

type AggSenderTx struct {
	Amount 				uint64
	Fee    				uint64
	TxCnt  				uint32
	From   				[32]byte
	To    				map[[32]byte]int
	AggregatedTxSlice 	[][32]byte
	Aggregated			bool
}

func ConstrAggSenderTx(amount uint64, fee uint64, txCnt uint32, from [32]byte, to [][32]byte, transactions [][32]byte) (tx *AggSenderTx, err error) {
	tx = new(AggSenderTx)
	tx.To = map[[32]byte]int{}

	tx.Amount = amount
	tx.From = from
	tx.Fee = fee
	tx.TxCnt = txCnt
	tx.AggregatedTxSlice = transactions
	tx.Aggregated = false

	for _, trx := range to {
		tx.To[trx] = tx.To[trx] + 1
	}

	return tx, nil
}


func (tx *AggSenderTx) Hash() (hash [32]byte) {
	if tx == nil {
		//is returning nil better?
		return [32]byte{}
	}

	txHash := struct {
		Amount			 	uint64
		Fee    				uint64
		TxCnt  				uint32
		From   				[32]byte
		To     				map[[32]byte]int
		AggregatedTxSlice 	[][32]byte

	}{
		tx.Amount,
		tx.Fee,
		tx.TxCnt,
		tx.From,
		tx.To,
		tx.AggregatedTxSlice,
	}

	return SerializeHashContent(txHash)
}

//when we serialize the struct with binary.Write, unexported field get serialized as well, undesired
//behavior. Therefore, writing own encoder/decoder
func (tx *AggSenderTx) Encode() (encodedTx []byte) {
	// Encode
	encodeData := AggSenderTx{
		Amount: 				tx.Amount,
		Fee:    				tx.Fee,
		TxCnt:  				tx.TxCnt,
		From:					tx.From,
		To:    					tx.To,
		AggregatedTxSlice: 		tx.AggregatedTxSlice,

	}
	buffer := new(bytes.Buffer)
	gob.NewEncoder(buffer).Encode(encodeData)
	return buffer.Bytes()
}

func (*AggSenderTx) Decode(encodedTx []byte) *AggSenderTx {
	var decoded AggSenderTx
	buffer := bytes.NewBuffer(encodedTx)
	decoder := gob.NewDecoder(buffer)
	decoder.Decode(&decoded)
	return &decoded
}

func (tx *AggSenderTx) TxFee() uint64 { return tx.Fee }
func (tx *AggSenderTx) Size() uint64  { return AGGSENDERTX_SIZE }

func (tx *AggSenderTx) Sender() [32]byte { return tx.From }
func (tx *AggSenderTx) Receiver() [32]byte { return [32]byte{} }


func (tx AggSenderTx) String() string {
	return fmt.Sprintf(
		"\nHash: %x\n" +
			"Amount: %v\n"+
			"Fee: %v\n"+
			"TxCnt: %v\n"+
			"From: %x\n"+
			"To: %x\n"+
			"Transactions: %x\n"+
			"#Tx: %v\n",
		tx.Hash(),
		tx.Amount,
		tx.Fee,
		tx.TxCnt,
		tx.From,
		tx.To,
		tx.AggregatedTxSlice,
		len(tx.AggregatedTxSlice),
	)
}

