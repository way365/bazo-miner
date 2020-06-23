package protocol

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/julwil/bazo-miner/crypto"
	"golang.org/x/crypto/sha3"
)

const (
	AGGTX_SIZE = 85 //Only constant Values --> Without To, From & AggregatedTxSlice
)

//when we broadcast transactions we need a way to distinguish with a type

type AggTx struct {
	Amount              uint64
	Fee                 uint64
	From                [][32]byte
	To                  [][32]byte
	AggregatedTxSlice   [][32]byte
	Aggregated          bool
	Block               [32]byte //This saves the blockHashWithoutTransactions into which the transaction was usually validated. Needed for rollback.
	MerkleRoot          [32]byte
	ChamHashCheckString *crypto.ChameleonHashCheckString // Chameleon hash check string associated with this tx.
	Data                []byte
}

func ConstrAggTx(amount uint64, fee uint64, from [][32]byte, to [][32]byte, transactions [][32]byte) (tx *AggTx, err error) {
	tx = new(AggTx)

	tx.Amount = amount
	tx.Fee = fee
	tx.From = from
	tx.To = to
	tx.AggregatedTxSlice = transactions
	tx.Aggregated = false
	tx.Block = [32]byte{}
	tx.MerkleRoot = BuildAggTxMerkleTree(transactions).MerkleRoot()

	return tx, nil
}

func (tx *AggTx) Hash() (hash [32]byte) {
	if tx == nil {
		//is returning nil better?
		return [32]byte{}
	}

	txHash := struct {
		Amount     uint64
		Fee        uint64
		From       [][32]byte
		To         [][32]byte
		MerkleRoot [32]byte
	}{
		tx.Amount,
		tx.Fee,
		tx.From,
		tx.To,
		tx.MerkleRoot,
	}

	return SerializeHashContent(txHash)
}

// Returns the chameleon hash but takes the chameleon hash parameters as input.
// This method should be called in the context of bazo-client as the client doesn't maintain
// a state holding the chameleon hash parameters of each account.
func (tx *AggTx) HashWithChamHashParams(chamHashParams *crypto.ChameleonHashParameters) [32]byte {
	sha3Hash := tx.SHA3()
	hashInput := sha3Hash[:]

	return crypto.ChameleonHash(chamHashParams, tx.ChamHashCheckString, &hashInput)
}

// Returns SHA3 hash over the tx content
func (tx *AggTx) SHA3() [32]byte {
	toHash := struct {
		Amount     uint64
		Fee        uint64
		From       [][32]byte
		To         [][32]byte
		MerkleRoot [32]byte
	}{
		tx.Amount,
		tx.Fee,
		tx.From,
		tx.To,
		tx.MerkleRoot,
	}

	return sha3.Sum256([]byte(fmt.Sprintf("%v", toHash)))
}

//when we serialize the struct with binary.Write, unexported field get serialized as well, undesired
//behavior. Therefore, writing own encoder/decoder
func (tx *AggTx) Encode() (encodedTx []byte) {
	// Encode
	encodeData := AggTx{
		Amount:            tx.Amount,
		Fee:               tx.Fee,
		From:              tx.From,
		To:                tx.To,
		AggregatedTxSlice: tx.AggregatedTxSlice,
		Aggregated:        tx.Aggregated,
		Block:             tx.Block,
		MerkleRoot:        tx.MerkleRoot,
	}
	buffer := new(bytes.Buffer)
	gob.NewEncoder(buffer).Encode(encodeData)
	return buffer.Bytes()
}

func (*AggTx) Decode(encodedTx []byte) *AggTx {
	var decoded AggTx
	buffer := bytes.NewBuffer(encodedTx)
	decoder := gob.NewDecoder(buffer)
	decoder.Decode(&decoded)
	return &decoded
}

func (tx *AggTx) TxFee() uint64 { return tx.Fee }
func (tx *AggTx) Size() uint64  { return AGGTX_SIZE }

func (tx *AggTx) Sender() [32]byte   { return [32]byte{} }
func (tx *AggTx) Receiver() [32]byte { return [32]byte{} }

func (tx AggTx) String() string {
	return fmt.Sprintf(
		"\n ________\n| AGGTX: |____________________________________________________________________\n"+
			"|  Hash: %x\n"+
			"|  Amount: %v\n"+
			"|  Fee: %v\n"+
			"|  From: %x\n"+
			"|  To: %x\n"+
			"|  Transactions: %x\n"+
			"|  Merkle Root: %x\n"+
			"|  #Tx: %v\n"+
			"|  Aggregated: %t\n"+
			"|_________________________________________________________________________________",
		tx.Hash(),
		tx.Amount,
		tx.Fee,
		tx.From,
		tx.To,
		tx.AggregatedTxSlice,
		tx.MerkleRoot,
		len(tx.AggregatedTxSlice),
		tx.Aggregated,
	)
}

func (tx *AggTx) SetData(data []byte) {
	tx.Data = data
}

func (tx *AggTx) SetChamHashCheckString(checkString *crypto.ChameleonHashCheckString) {
	tx.ChamHashCheckString = checkString
}

func (tx *AggTx) GetChamHashCheckString() *crypto.ChameleonHashCheckString {
	return tx.ChamHashCheckString
}
