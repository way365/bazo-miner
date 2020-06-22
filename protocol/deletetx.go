package protocol

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"github.com/julwil/bazo-miner/crypto"
)

const DELETE_TX_SIZE = 42

type DeleteTx struct {
	Header              byte
	Fee                 uint64
	TxToDeleteHash      [32]byte                         // The hash of the tx to be deleted.
	Issuer              [32]byte                         // The address of the issuer of the deletion request.
	Sig                 [64]byte                         // The signature of the issuer of the deletion request.
	ChamHashCheckString *crypto.ChameleonHashCheckString // Chameleon hash check string associated with this tx.

	Data []byte // Data field for user-related data.
}

func ConstrDeleteTx(
	header byte,
	fee uint64,
	txToDeleteHash [32]byte,
	issuer [32]byte,
	privateKey *ecdsa.PrivateKey,
) (tx *DeleteTx, err error) {
	tx = new(DeleteTx)
	tx.Header = header
	tx.Fee = fee
	tx.TxToDeleteHash = txToDeleteHash
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

func (tx *DeleteTx) Hash() (hash [32]byte) {
	if tx == nil {

		return [32]byte{}
	}

	txHash := struct {
		Header         byte
		Fee            uint64
		TxToDeleteHash [32]byte
		Issuer         [32]byte
	}{
		tx.Header,
		tx.Fee,
		tx.TxToDeleteHash,
		tx.Issuer,
	}

	return SerializeHashContent(txHash)
}

func (tx *DeleteTx) Encode() (encodedTx []byte) {
	encodeData := DeleteTx{
		Header:         tx.Header,
		Fee:            tx.Fee,
		TxToDeleteHash: tx.TxToDeleteHash,
		Issuer:         tx.Issuer,
		Sig:            tx.Sig,
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
			"Issuer: %x\n"+
			"Sig: %x\n",
		tx.Header,
		tx.Fee,
		tx.TxToDeleteHash,
		tx.Issuer[0:8],
		tx.Sig[0:8],
	)
}

func (tx *DeleteTx) SetData(data []byte) {
	tx.Data = data
}
