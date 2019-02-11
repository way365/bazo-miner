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
	Header 				byte
	Amount 				uint64
	Fee    				uint64
	TxCnt  				uint32
	From   				[32]byte
	ToSlice    			[]byte
	AggregatedTxSlice 	[]byte
}

func ConstrAggTxSender(header byte, amount uint64, fee uint64, txCnt uint32, from [32]byte, to []byte) (tx *AggTxSender, err error) {
	tx = new(AggTxSender)

	tx.Header = header
	tx.From = from
	tx.ToSlice = to
	tx.Amount = amount
	tx.Fee = fee
	tx.TxCnt = txCnt

	txHash := tx.Hash()

	txHash = txHash

	return tx, nil
}


func (tx *AggTxSender) Hash() (hash [32]byte) {
	if tx == nil {
		//is returning nil better?
		return [32]byte{}
	}

	txHash := struct {
		Header byte
		Amount uint64
		Fee    uint64
		TxCnt  uint32
		From   [32]byte
		To     []byte
	}{
		tx.Header,
		tx.Amount,
		tx.Fee,
		tx.TxCnt,
		tx.From,
		tx.ToSlice,
	}

	return SerializeHashContent(txHash)
}

//when we serialize the struct with binary.Write, unexported field get serialized as well, undesired
//behavior. Therefore, writing own encoder/decoder
func (tx *AggTxSender) Encode() (encodedTx []byte) {
	// Encode
	encodeData := AggTxSender{
		Header: 	tx.Header,
		Amount: 	tx.Amount,
		Fee:    	tx.Fee,
		TxCnt:  	tx.TxCnt,
		From:		tx.From,
		ToSlice:    tx.ToSlice,
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
func (tx *AggTxSender) ReceiverSlice() []byte { return tx.ToSlice }

func (tx AggTxSender) String() string {
	return fmt.Sprintf(
		"\nHeader: %v\n"+
			"Amount: %v\n"+
			"Fee: %v\n"+
			"TxCnt: %v\n"+
			"From: %x\n"+
			"To: %x\n",
		tx.Header,
		tx.Amount,
		tx.Fee,
		tx.TxCnt,
		tx.From,
		tx.ToSlice,
	)
}
