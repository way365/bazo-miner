package protocol

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

const (
	AGGTX_SENDER_SIZE = 213 //TODO correct size
)

//when we broadcast transactions we need a way to distinguish with a type

type AggTxSender struct {
	Amount 				uint64
	Fee    				uint64
	TxCnt  				uint32
	From   				[32]byte
	To    				map[[32]byte][32]byte
	AggregatedTxSlice 	[][32]byte
	Aggregated			bool
}

func ConstrAggTxSender(amount uint64, fee uint64, txCnt uint32, from [32]byte, to [][32]byte, transactions [][32]byte) (tx *AggTxSender, err error) {
	tx = new(AggTxSender)
	tx.To = map[[32]byte][32]byte{}

	tx.Amount = amount
	tx.From = from
	tx.Fee = fee
	tx.TxCnt = txCnt
	tx.AggregatedTxSlice = transactions
	tx.Aggregated = false

	for _, trx := range to {
		tx.To[trx] = trx
	}

	return tx, nil
}


func (tx *AggTxSender) Hash() (hash [32]byte) {
	if tx == nil {
		//is returning nil better?
		return [32]byte{}
	}

	txHash := struct {
		Amount			 	uint64
		Fee    				uint64
		TxCnt  				uint32
		From   				[32]byte
		To     				map[[32]byte][32]byte
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
func (tx *AggTxSender) Encode() (encodedTx []byte) {
	// Encode
	encodeData := AggTxSender{
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

func (*AggTxSender) Decode(encodedTx []byte) *AggTxSender {
	var decoded AggTxSender
	buffer := bytes.NewBuffer(encodedTx)
	decoder := gob.NewDecoder(buffer)
	decoder.Decode(&decoded)
	return &decoded
}

func (tx *AggTxSender) TxFee() uint64 { return tx.Fee }
func (tx *AggTxSender) Size() uint64  { return AGGTX_SENDER_SIZE }

func (tx *AggTxSender) Sender() [32]byte { return tx.From }

func (tx AggTxSender) String() string {
	return fmt.Sprintf(
		"\nHash: %x\n" +
			"Amount: %v\n"+
			"Fee: %v\n"+
			"TxCnt: %v\n"+
			"From: %x\n"+
			"To: %x\n"+
			"Transactions: %x\n",
		tx.Hash(),
		tx.Amount,
		tx.Fee,
		tx.TxCnt,
		tx.From,
		tx.To,
		tx.AggregatedTxSlice,
	)
}

