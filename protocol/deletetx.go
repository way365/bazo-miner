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

const DELETE_TX_SIZE = 42

type DeleteTx struct {
	Header                        byte
	Fee                           uint64
	TxToDeleteHash                [32]byte                         // Hash of the tx to be deleted.
	TxToDeleteChamHashCheckString *crypto.ChameleonHashCheckString // New Chameleon hash check string for the tx to be deleted.
	TxToDeleteData                []byte                           // Holds the data to be updated on the TxToUpdate's data field.
	Issuer                        [32]byte                         // Address of the issuer of the deletion request.
	Sig                           [64]byte                         // Signature of the issuer of the deletion request.
	ChamHashCheckString           *crypto.ChameleonHashCheckString // Chameleon hash check string associated with this tx.

	Data []byte // Data field for user-related data.
}

func ConstrDeleteTx(
	header byte,
	fee uint64,
	txToDeleteHash [32]byte,
	txToDeleteChamHashCheckString *crypto.ChameleonHashCheckString,
	txToDeleteData []byte,
	issuer [32]byte,
	privateKey *ecdsa.PrivateKey,
) (tx *DeleteTx, err error) {
	tx = new(DeleteTx)
	tx.Header = header
	tx.Fee = fee
	tx.TxToDeleteHash = txToDeleteHash
	tx.TxToDeleteChamHashCheckString = txToDeleteChamHashCheckString
	tx.TxToDeleteData = txToDeleteData
	tx.Issuer = issuer

	// Generate the hash of the new Tx
	txHash := tx.Hash()

	// Sign the Tx
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, txHash[:])
	if err != nil {
		return nil, err
	}
	copy(tx.Sig[32-len(r.Bytes()):32], r.Bytes())
	copy(tx.Sig[64-len(s.Bytes()):], s.Bytes())

	return tx, err
}

// Returns SHA3 hash over the tx content
func (tx *DeleteTx) SHA3() [32]byte {
	toHash := struct {
		Header                        byte
		Fee                           uint64
		TxToDeleteHash                [32]byte
		TxToDeleteChamHashCheckString *crypto.ChameleonHashCheckString
		TxToDeleteData                []byte
		Issuer                        [32]byte
	}{
		tx.Header,
		tx.Fee,
		tx.TxToDeleteHash,
		tx.TxToDeleteChamHashCheckString,
		tx.TxToDeleteData,
		tx.Issuer,
	}

	return sha3.Sum256([]byte(fmt.Sprintf("%v", toHash)))
}

func (tx *DeleteTx) Hash() (hash [32]byte) {
	if tx == nil {

		return [32]byte{}
	}

	txHash := struct {
		Header                        byte
		Fee                           uint64
		TxToDeleteHash                [32]byte
		TxToDeleteChamHashCheckString crypto.ChameleonHashCheckString
		TxToDeleteData                []byte
		Issuer                        [32]byte
	}{
		tx.Header,
		tx.Fee,
		tx.TxToDeleteHash,
		*tx.TxToDeleteChamHashCheckString, // Important: We need the value, not the pointer here!
		tx.TxToDeleteData,
		tx.Issuer,
	}

	return SerializeHashContent(txHash)
}

// Returns the chameleon hash but takes the chameleon hash parameters as input.
// This method should be called in the context of bazo-client as the client doesn't maintain
// a state holding the chameleon hash parameters of each account.
func (tx *DeleteTx) HashWithChamHashParams(chamHashParams *crypto.ChameleonHashParameters) [32]byte {
	sha3Hash := tx.SHA3()
	hashInput := sha3Hash[:]

	return crypto.ChameleonHash(chamHashParams, tx.ChamHashCheckString, &hashInput)
}

func (tx *DeleteTx) Encode() (encodedTx []byte) {
	encodeData := DeleteTx{
		Header:                        tx.Header,
		Fee:                           tx.Fee,
		TxToDeleteHash:                tx.TxToDeleteHash,
		TxToDeleteChamHashCheckString: tx.TxToDeleteChamHashCheckString,
		TxToDeleteData:                tx.TxToDeleteData,
		Issuer:                        tx.Issuer,
		Sig:                           tx.Sig,
	}
	buffer := new(bytes.Buffer)
	gob.NewEncoder(buffer).Encode(encodeData)

	return buffer.Bytes()
}

func (*DeleteTx) Decode(encodedTx []byte) *DeleteTx {
	var decoded DeleteTx
	buffer := bytes.NewBuffer(encodedTx)
	decoder := gob.NewDecoder(buffer)
	decoder.Decode(&decoded)

	return &decoded
}

func (tx *DeleteTx) TxFee() uint64 { return tx.Fee }

func (tx *DeleteTx) Size() uint64 { return DELETE_TX_SIZE }

func (tx *DeleteTx) Sender() [32]byte { return tx.Issuer }

func (tx *DeleteTx) Receiver() [32]byte { return tx.Issuer }

func (tx DeleteTx) String() string {

	return fmt.Sprintf(
		"\nHeader: %v\n"+
			"Fee: %v\n"+
			"TxToDelete: %x\n"+
			"TxToDeleteChamHashCheckString: %x\n"+
			"TxToDeleteData: %s\n"+
			"Issuer: %x\n"+
			"Sig: %x\n",
		tx.Header,
		tx.Fee,
		tx.TxToDeleteHash,
		tx.TxToDeleteChamHashCheckString.R[0:8],
		tx.TxToDeleteData,
		tx.Issuer[0:8],
		tx.Sig[0:8],
	)
}

func (tx *DeleteTx) SetData(data []byte) {
	tx.Data = data
}

func (tx *DeleteTx) SetChamHashCheckString(checkString *crypto.ChameleonHashCheckString) {
	tx.ChamHashCheckString = checkString
}

func (tx *DeleteTx) GetChamHashCheckString() *crypto.ChameleonHashCheckString {
	return tx.ChamHashCheckString
}
