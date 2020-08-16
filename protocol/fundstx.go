package protocol

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/julwil/bazo-miner/crypto"
	"golang.org/x/crypto/sha3"
)

const (
	FUNDSTX_SIZE = 246
)

//when we broadcast transactions we need a way to distinguish with a type

type FundsTx struct {
	Header      byte
	Amount      uint64
	Fee         uint64
	TxCnt       uint32
	From        [32]byte
	To          [32]byte
	Sig1        [64]byte
	Sig2        [64]byte
	Aggregated  bool
	Block       [32]byte                         // This saves the blockHashWithoutTransactions into which the transaction was usually validated. Needed for rollback.
	CheckString *crypto.ChameleonHashCheckString // Chameleon hash check string associated with this tx.
	Data        []byte
}

func ConstrFundsTx(
	header byte,
	amount uint64,
	fee uint64,
	txCnt uint32,
	from, to [32]byte,
	checkString *crypto.ChameleonHashCheckString,
	data []byte,
) (tx *FundsTx, err error) {
	tx = new(FundsTx)

	tx.Header = header
	tx.From = from
	tx.To = to
	tx.Amount = amount
	tx.Fee = fee
	tx.TxCnt = txCnt
	tx.Aggregated = false
	tx.Data = data
	tx.Block = [32]byte{}
	tx.CheckString = checkString

	return tx, nil
}

// Returns the chameleon hash but takes the chameleon hash parameters as input.
// This method should be called in the context of bazo-client as the client doesn't maintain
// a copy of the chameleon hash parameters of each account.
func (tx *FundsTx) ChameleonHash(parameters *crypto.ChameleonHashParameters) [32]byte {
	sha3Hash := tx.SHA3()
	hashInput := sha3Hash[:]

	return crypto.ChameleonHash(parameters, tx.CheckString, &hashInput)
}

// Returns the chameleon hash without chameleon hash parameters as input.
// This can be called in the context of bazo-miner as the miner keeps a state
// with the chameleon hash parameters of all accounts.
func (tx *FundsTx) Hash() (hash [32]byte) {
	if tx == nil {
		return [32]byte{}
	}

	parameters := crypto.ChameleonHashParametersMap[tx.From]

	return tx.ChameleonHash(parameters)
}

// Returns SHA3 hash over the tx content
func (tx *FundsTx) SHA3() [32]byte {
	toHash := struct {
		Header byte
		Amount uint64
		Fee    uint64
		TxCnt  uint32
		From   [32]byte
		To     [32]byte
		Data   []byte
	}{
		tx.Header,
		tx.Amount,
		tx.Fee,
		tx.TxCnt,
		tx.From,
		tx.To,
		tx.Data,
	}

	return sha3.Sum256([]byte(fmt.Sprintf("%v", toHash)))
}

//when we serialize the struct with binary.Write, unexported field get serialized as well, undesired
//behavior. Therefore, writing own encoder/decoder
func (tx *FundsTx) Encode() (encodedTx []byte) {
	// Encode
	encodeData := FundsTx{
		Header:      tx.Header,
		Amount:      tx.Amount,
		Fee:         tx.Fee,
		TxCnt:       tx.TxCnt,
		From:        tx.From,
		To:          tx.To,
		Sig1:        tx.Sig1,
		Sig2:        tx.Sig2,
		Data:        tx.Data,
		Aggregated:  tx.Aggregated,
		Block:       tx.Block,
		CheckString: tx.CheckString,
	}
	buffer := new(bytes.Buffer)
	gob.NewEncoder(buffer).Encode(encodeData)
	return buffer.Bytes()
}

func (*FundsTx) Decode(encodedTx []byte) *FundsTx {
	var decoded FundsTx
	buffer := bytes.NewBuffer(encodedTx)
	decoder := gob.NewDecoder(buffer)
	decoder.Decode(&decoded)
	return &decoded
}

func (tx *FundsTx) TxFee() uint64 { return tx.Fee }
func (tx *FundsTx) Size() uint64  { return FUNDSTX_SIZE }

func (tx *FundsTx) Sender() [32]byte   { return tx.From }
func (tx *FundsTx) Receiver() [32]byte { return tx.To }

func (tx FundsTx) String() string {
	return fmt.Sprintf(
		"\nHeader: %v\n"+
			"Amount: %v\n"+
			"Fee: %v\n"+
			"TxCnt: %v\n"+
			"From: %x\n"+
			"To: %x\n"+
			"Sig1: %x\n"+
			"Sig2: %x\n"+
			"Aggregated: %t\n"+
			"CheckString: %x\n"+
			"Data: %s",
		tx.Header,
		tx.Amount,
		tx.Fee,
		tx.TxCnt,
		tx.From[0:8],
		tx.To[0:8],
		tx.Sig1[0:8],
		tx.Sig2[0:8],
		tx.Aggregated,
		tx.CheckString.R[0:8],
		tx.Data,
	)
}

func (tx *FundsTx) SetData(data []byte) {
	tx.Data = data
}

func (tx *FundsTx) GetData() []byte {
	return tx.Data
}

func (tx *FundsTx) SetCheckString(checkString *crypto.ChameleonHashCheckString) {
	tx.CheckString = checkString
}

func (tx *FundsTx) GetCheckString() *crypto.ChameleonHashCheckString {
	return tx.CheckString
}

func (tx *FundsTx) SetSignature(signature [64]byte) {
	tx.Sig1 = signature
}
