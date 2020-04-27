package protocol

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/gob"
	"fmt"
)

const (
	DELETE_TX_SIZE = 42 //TODO: Define size here
)

type DeleteTx struct {
	Header         byte
	Fee            uint64
	TxToDeleteHash [32]byte // The hash of the tx to be deleted
	Issuer         [32]byte // The address of the issuer of the deletion request.
	Sig            [64]byte // The signature of the issuer of the deletion request.
	TxCnt          uint32
}

func ConstrDeleteTx(
	header byte,
	fee uint64,
	txToDeleteHash [32]byte,
	issuer [32]byte,
	sigKey *ecdsa.PrivateKey,
	txCnt uint32,
) (tx *DeleteTx, err error) {
	tx = new(DeleteTx)
	tx.Header = header
	tx.Fee = fee
	tx.TxToDeleteHash = txToDeleteHash
	tx.Issuer = issuer
	tx.TxCnt = txCnt

	// Generate the hash of the new Tx
	txHash := tx.Hash()

	// Sign the Tx
	r, s, err := ecdsa.Sign(rand.Reader, sigKey, txHash[:])
	if err != nil {
		return nil, err
	}
	copy(tx.Sig[32-len(r.Bytes()):32], r.Bytes())
	copy(tx.Sig[64-len(s.Bytes()):], s.Bytes())

	return tx, nil
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
		Sig            [64]byte
		TxCnt          uint32
	}{
		tx.Header,
		tx.Fee,
		tx.TxToDeleteHash,
		tx.Issuer,
		tx.Sig,
		tx.TxCnt,
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
		TxCnt:          tx.TxCnt,
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
			"Sig: %x\n"+
			"TxCnt: %v\n",
		tx.Header,
		tx.Fee,
		tx.TxToDeleteHash,
		tx.Issuer[0:8],
		tx.Sig[0:8],
		tx.TxCnt,
	)
}
