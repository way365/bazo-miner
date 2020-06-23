package protocol

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
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
	Header              byte
	Amount              uint64
	Fee                 uint64
	TxCnt               uint32
	From                [32]byte
	To                  [32]byte
	Sig1                [64]byte
	Sig2                [64]byte
	Aggregated          bool
	Block               [32]byte                         // This saves the blockHashWithoutTransactions into which the transaction was usually validated. Needed for rollback.
	ChamHashCheckString *crypto.ChameleonHashCheckString // Chameleon hash check string associated with this tx.

	Data []byte
}

func ConstrFundsTx(
	header byte,
	amount uint64,
	fee uint64,
	txCnt uint32,
	from, to [32]byte,
	sig1Key *ecdsa.PrivateKey,
	sig2Key *ecdsa.PrivateKey,
	chamHashCheckString *crypto.ChameleonHashCheckString,
	chamHashParams *crypto.ChameleonHashParameters,
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
	tx.ChamHashCheckString = chamHashCheckString

	txHash := tx.HashWithChamHashParams(chamHashParams)

	r, s, err := ecdsa.Sign(rand.Reader, sig1Key, txHash[:])
	if err != nil {
		return nil, err
	}

	copy(tx.Sig1[32-len(r.Bytes()):32], r.Bytes())
	copy(tx.Sig1[64-len(s.Bytes()):], s.Bytes())

	if sig2Key != nil {
		r, s, err := ecdsa.Sign(rand.Reader, sig2Key, txHash[:])
		if err != nil {
			return nil, err
		}

		copy(tx.Sig2[32-len(r.Bytes()):32], r.Bytes())
		copy(tx.Sig2[64-len(s.Bytes()):], s.Bytes())
	}

	return tx, nil
}

// Returns the chameleon hash but takes the chameleon hash parameters as input.
// This method should be called in the context of bazo-client as the client doesn't maintain
// a state holding the chameleon hash parameters of each account.
func (tx *FundsTx) HashWithChamHashParams(chamHashParams *crypto.ChameleonHashParameters) [32]byte {
	sha3Hash := tx.SHA3()
	hashInput := sha3Hash[:]
	fmt.Printf("ChamHash Parameters: %x", chamHashParams.HK[0:8])
	fmt.Printf("ChamHash Check String: %x", tx.ChamHashCheckString.R[0:8])

	return crypto.ChameleonHash(chamHashParams, tx.ChamHashCheckString, &hashInput)
}

// Returns the chameleon hash without chameleon hash parameters as input.
// This can be called in the context of bazo-miner as the miner keeps a state
// with the chameleon hash parameters of all accounts.
func (tx *FundsTx) Hash() (hash [32]byte) {
	chamHashParams := crypto.ChamHashParamsMap[tx.From]

	return tx.HashWithChamHashParams(chamHashParams)
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
		Header:              tx.Header,
		Amount:              tx.Amount,
		Fee:                 tx.Fee,
		TxCnt:               tx.TxCnt,
		From:                tx.From,
		To:                  tx.To,
		Sig1:                tx.Sig1,
		Sig2:                tx.Sig2,
		Data:                tx.Data,
		Aggregated:          tx.Aggregated,
		Block:               tx.Block,
		ChamHashCheckString: tx.ChamHashCheckString,
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
			"Data: %s\n"+
			"Aggregated: %t\n"+
			"ChamHashCheckString: %x",
		tx.Header,
		tx.Amount,
		tx.Fee,
		tx.TxCnt,
		tx.From[0:8],
		tx.To[0:8],
		tx.Sig1[0:8],
		tx.Sig2[0:8],
		tx.Data,
		tx.Aggregated,
		tx.ChamHashCheckString.R[0:8],
	)
}

func (tx *FundsTx) SetData(data []byte) {
	tx.Data = data
}

func (tx *FundsTx) SetChamHashCheckString(checkString *crypto.ChameleonHashCheckString) {
	tx.ChamHashCheckString = checkString
}

func (tx *FundsTx) GetChamHashCheckString() *crypto.ChameleonHashCheckString {
	return tx.ChamHashCheckString
}
