package protocol

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"encoding/binary"
	"fmt"
	"github.com/way365/bazo-miner/crypto"
)

const (
	STAKETX_SIZE = 106 + crypto.COMM_KEY_LENGTH
)

//when we broadcast transactions we need a way to distinguish with a type

type StakeTx struct {
	Header        byte                             // 1 Byte
	Fee           uint64                           // 8 Byte
	IsStaking     bool                             // 1 Byte
	Account       [32]byte                         // 32 Byte
	Sig           [64]byte                         // 64 Byte
	CommitmentKey [crypto.COMM_KEY_LENGTH]byte     // the modulus N of the RSA public key
	CheckString   *crypto.ChameleonHashCheckString // Chameleon hash check string associated with this tx.
	Data          []byte
}

func ConstrStakeTx(header byte, fee uint64, isStaking bool, account [32]byte, signKey *ecdsa.PrivateKey, commPubKey *rsa.PublicKey) (tx *StakeTx, err error) {

	tx = new(StakeTx)

	tx.Header = header
	tx.Fee = fee
	tx.IsStaking = isStaking
	tx.Account = account

	copy(tx.CommitmentKey[:], commPubKey.N.Bytes())

	txHash := tx.Hash()

	r, s, err := ecdsa.Sign(rand.Reader, signKey, txHash[:])
	if err != nil {
		return nil, err
	}

	copy(tx.Sig[32-len(r.Bytes()):32], r.Bytes())
	copy(tx.Sig[64-len(s.Bytes()):], s.Bytes())

	return tx, nil
}

// Returns SHA3 hash over the tx content
func (tx *StakeTx) SHA3() [32]byte {

	return tx.Hash()
}

func (tx *StakeTx) Hash() (hash [32]byte) {
	if tx == nil {
		//is returning nil better?
		return [32]byte{}
	}

	txHash := struct {
		Header    byte
		Fee       uint64
		IsStaking bool
		Account   [32]byte
		CommKey   [crypto.COMM_KEY_LENGTH]byte
	}{
		tx.Header,
		tx.Fee,
		tx.IsStaking,
		tx.Account,
		tx.CommitmentKey,
	}

	return SerializeHashContent(txHash)
}

// As we don't use chameleon hashing on config tx, we simply return an SHA3 hash
func (tx *StakeTx) ChameleonHash(parameters *crypto.ChameleonHashParameters) [32]byte {

	return tx.Hash()
}

// when we serialize the struct with binary.Write, unexported field get serialized as well, undesired
// behavior. Therefore, writing own encoder/decoder
func (tx *StakeTx) Encode() (encodedTx []byte) {
	if tx == nil {
		return nil
	}

	var fee [8]byte
	var isStaking byte

	binary.BigEndian.PutUint64(fee[:], tx.Fee)

	if tx.IsStaking == true {
		isStaking = 1
	} else {
		isStaking = 0
	}

	encodedTx = make([]byte, STAKETX_SIZE)

	encodedTx[0] = tx.Header
	copy(encodedTx[1:9], fee[:])
	encodedTx[9] = isStaking
	copy(encodedTx[10:42], tx.Account[:])
	copy(encodedTx[42:106], tx.Sig[:])
	copy(encodedTx[106:106+crypto.COMM_KEY_LENGTH], tx.CommitmentKey[:])

	return encodedTx
}

func (*StakeTx) Decode(encodedTx []byte) (tx *StakeTx) {
	tx = new(StakeTx)

	if len(encodedTx) != STAKETX_SIZE {
		return nil
	}

	var isStakingAsByte byte

	tx.Header = encodedTx[0]
	tx.Fee = binary.BigEndian.Uint64(encodedTx[1:9])
	isStakingAsByte = encodedTx[9]
	copy(tx.Account[:], encodedTx[10:42])
	copy(tx.Sig[:], encodedTx[42:106])
	copy(tx.CommitmentKey[:], encodedTx[106:106+crypto.COMM_KEY_LENGTH])

	if isStakingAsByte == 0 {
		tx.IsStaking = false
	} else {
		tx.IsStaking = true
	}

	return tx
}

func (tx *StakeTx) TxFee() uint64      { return tx.Fee }
func (tx *StakeTx) Size() uint64       { return STAKETX_SIZE }
func (tx *StakeTx) Sender() [32]byte   { return [32]byte{} } //return empty because it is not needed.
func (tx *StakeTx) Receiver() [32]byte { return [32]byte{} }

func (tx StakeTx) String() string {
	return fmt.Sprintf(
		"\nHeader: %x\n"+
			"Fee: %v\n"+
			"IsStaking: %v\n"+
			"Account: %x\n"+
			"Sig: %x\n"+
			"CommitmentKey: %x\n",
		tx.Header,
		tx.Fee,
		tx.IsStaking,
		tx.Account[0:8],
		tx.Sig[0:8],
		tx.CommitmentKey[0:8],
	)
}

func (tx *StakeTx) SetData(data []byte) {
	tx.Data = data
}

func (tx *StakeTx) GetData() []byte {
	return tx.Data
}

func (tx *StakeTx) SetCheckString(checkString *crypto.ChameleonHashCheckString) {
	tx.CheckString = checkString
}

func (tx *StakeTx) GetCheckString() *crypto.ChameleonHashCheckString {
	return tx.CheckString
}

func (tx *StakeTx) SetSignature(signature [64]byte) {
	tx.Sig = signature
}
