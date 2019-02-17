package protocol

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

const (
	AGGRECEIVERTX_SIZE = 213 //TODO correct size
)

//when we broadcast transactions we need a way to distinguish with a type

type AggReceiverTx struct {
	Amount 				uint64
	Fee    				uint64
	To   				[32]byte
	From    			map[[32]byte]uint32
	AggregatedTxSlice 	[][32]byte
	Aggregated			bool
}

func ConstrAggReceiverTx(amount uint64, fee uint64, from [][32]byte, to [32]byte, transactions [][32]byte) (tx *AggReceiverTx, err error) {
	tx = new(AggReceiverTx)
	tx.From = map[[32]byte]uint32{}
	tx.Amount = amount
	tx.To = to
	tx.Fee = fee
	tx.AggregatedTxSlice = transactions
	tx.Aggregated = false

	for _, trx := range from {
		tx.From[trx] = tx.From[trx]+1
	}

	return tx, nil
}


func (tx *AggReceiverTx) Hash() (hash [32]byte) {
	if tx == nil {
		//is returning nil better?
		return [32]byte{}
	}

	txHash := struct {
		Amount			 	uint64
		Fee    				uint64
		to   				[32]byte
		From     			map[[32]byte]uint32
		AggregatedTxSlice 	[][32]byte

	}{
		tx.Amount,
		tx.Fee,
		tx.To,
		tx.From,
		tx.AggregatedTxSlice,
	}

	return SerializeHashContent(txHash)
}

//when we serialize the struct with binary.Write, unexported field get serialized as well, undesired
//behavior. Therefore, writing own encoder/decoder
func (tx *AggReceiverTx) Encode() (encodedTx []byte) {
	// Encode
	encodeData := AggReceiverTx{
		Amount: 				tx.Amount,
		Fee:    				tx.Fee,
		To:						tx.To,
		From:    				tx.From,
		AggregatedTxSlice: 		tx.AggregatedTxSlice,

	}
	buffer := new(bytes.Buffer)
	gob.NewEncoder(buffer).Encode(encodeData)
	return buffer.Bytes()
}

func (*AggReceiverTx) Decode(encodedTx []byte) *AggReceiverTx {
	var decoded AggReceiverTx
	buffer := bytes.NewBuffer(encodedTx)
	decoder := gob.NewDecoder(buffer)
	decoder.Decode(&decoded)
	return &decoded
}

func (tx *AggReceiverTx) TxFee() uint64 { return tx.Fee }
func (tx *AggReceiverTx) Size() uint64  { return AGGRECEIVERTX_SIZE }

func (tx *AggReceiverTx) Sender() [32]byte { return [32]byte{} }
func (tx *AggReceiverTx) Receiver() [32]byte { return tx.To }


func (tx AggReceiverTx) String() string {
	return fmt.Sprintf(
		"\nHash: %x\n" +
			"Amount: %v\n"+
			"Fee: %v\n"+
			"To: %x\n"+
			"From: %x\n"+
			"Transactions: %x\n"+
			"#Tx: %v\n",
		tx.Hash(),
		tx.Amount,
		tx.Fee,
		tx.To,
		tx.From,
		tx.AggregatedTxSlice,
		len(tx.AggregatedTxSlice),
	)
}

