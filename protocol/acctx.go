package protocol

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/julwil/bazo-miner/crypto"
	"golang.org/x/crypto/sha3"
)

const (
	ACCTX_SIZE = 169
)

type AccTx struct {
	Header            byte
	Fee               uint64
	Issuer            [32]byte
	PubKey            [64]byte
	Sig               [64]byte
	Contract          []byte
	ContractVariables []ByteArray
	ChParams          *crypto.ChameleonHashParameters  // Chameleon hash parameters associated with this account.
	ChCheckString     *crypto.ChameleonHashCheckString // Chameleon hash check string associated with this tx.
	Data              []byte
}

func ConstrAccTx(
	header byte,
	fee uint64,
	issuer [32]byte,
	address [64]byte,
	contract []byte,
	contractVariables []ByteArray,
	chParams *crypto.ChameleonHashParameters,
	chCheckString *crypto.ChameleonHashCheckString,
	data []byte,
) (tx *AccTx, err error) {
	tx = new(AccTx)
	tx.Header = header
	tx.Fee = fee
	tx.Issuer = issuer
	tx.PubKey = address
	tx.Contract = contract
	tx.ContractVariables = contractVariables
	tx.ChParams = chParams
	tx.ChCheckString = chCheckString
	tx.Data = data

	//if address != [64]byte{} {
	//	copy(tx.PubKey[:], address[:])
	//} else {
	//	var newAccAddressString string
	//	//Check if string representation of account address is 128 long. Else there will be problems when doing REST calls.
	//	for len(newAccAddressString) != 128 {
	//		newAccAddress, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	//		newAccPub1, newAccPub2 := newAccAddress.PublicKey.X.Bytes(), newAccAddress.PublicKey.Y.Bytes()
	//		copy(tx.PubKey[32-len(newAccPub1):32], newAccPub1)
	//		copy(tx.PubKey[64-len(newAccPub2):], newAccPub2)
	//
	//		newAccAddressString = newAccAddress.X.Text(16) + newAccAddress.Y.Text(16)
	//	}
	//}

	return tx, nil
}

// Returns SHA3 hash over the tx content
func (tx *AccTx) SHA3() [32]byte {
	toHash := struct {
		Header            byte
		Issuer            [32]byte
		Fee               uint64
		PubKey            [64]byte
		Contract          []byte
		ContractVariables []ByteArray
		Data              []byte
	}{
		tx.Header,
		tx.Issuer,
		tx.Fee,
		tx.PubKey,
		tx.Contract,
		tx.ContractVariables,
		tx.Data,
	}

	return sha3.Sum256([]byte(fmt.Sprintf("%v", toHash)))
}

func (tx *AccTx) Hash() [32]byte {
	if tx == nil {
		return [32]byte{}
	}

	return tx.ChameleonHash(tx.ChParams)
}

// Returns the chameleon hash but takes the chameleon hash parameters as input.
// This method should be called in the context of bazo-client as the client doesn't maintain
// a state holding the chameleon hash parameters of each account.
func (tx *AccTx) ChameleonHash(chParams *crypto.ChameleonHashParameters) [32]byte {
	sha3Hash := tx.SHA3()
	hashInput := sha3Hash[:]

	return crypto.ChameleonHash(chParams, tx.ChCheckString, &hashInput)
}

func (tx *AccTx) Encode() []byte {
	if tx == nil {
		return nil
	}

	encoded := AccTx{
		Header:        tx.Header,
		Issuer:        tx.Issuer,
		Fee:           tx.Fee,
		PubKey:        tx.PubKey,
		Sig:           tx.Sig,
		ChParams:      tx.ChParams,
		ChCheckString: tx.ChCheckString,
		Data:          tx.Data,
	}

	buffer := new(bytes.Buffer)
	gob.NewEncoder(buffer).Encode(encoded)
	return buffer.Bytes()
}

func (*AccTx) Decode(encoded []byte) (tx *AccTx) {
	var decoded AccTx
	buffer := bytes.NewBuffer(encoded)
	decoder := gob.NewDecoder(buffer)
	decoder.Decode(&decoded)
	return &decoded
}

func (tx *AccTx) TxFee() uint64 { return tx.Fee }

func (tx *AccTx) Size() uint64 { return ACCTX_SIZE }

func (tx *AccTx) Sender() [32]byte   { return tx.Issuer }
func (tx *AccTx) Receiver() [32]byte { return [32]byte{} }

func (tx AccTx) String() string {
	return fmt.Sprintf(
		"\n"+
			"Header: %x\n"+
			"Issuer: %x\n"+
			"Fee: %v\n"+
			"PubKey: %x\n"+
			"Sig: %x\n"+
			"Contract: %v\n"+
			"ContractVariables: %v\n"+
			"Data: %s",
		tx.Header,
		tx.Issuer[0:8],
		tx.Fee,
		tx.PubKey[0:8],
		tx.Sig[0:8],
		tx.Contract[:],
		tx.ContractVariables[:],
		tx.Data,
	)
}

func (tx *AccTx) SetData(data []byte) {
	tx.Data = data
}

func (tx *AccTx) GetData() []byte {
	return tx.Data
}

func (tx *AccTx) SetChCheckString(checkString *crypto.ChameleonHashCheckString) {
	tx.ChCheckString = checkString
}

func (tx *AccTx) GetChCheckString() *crypto.ChameleonHashCheckString {
	return tx.ChCheckString
}

func (tx *AccTx) SetSignature(signature [64]byte) {
	tx.Sig = signature
}
