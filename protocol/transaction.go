package protocol

const (
	TRANSACTION_HASH_SIZE     = 32
	TRANSACTION_SENDER_SIZE   = 32
	TRANSACTION_RECEIVER_SIZE = 32
)

type Transaction interface {
	Hash() [TRANSACTION_HASH_SIZE]byte
	Encode() []byte
	//Decoding is not listed here, because it returns a different type for each tx (return value Transaction itself
	//is apparently not allowed)
	TxFee() uint64
	Size() uint64
	Sender() [TRANSACTION_SENDER_SIZE]byte
	Receiver() [TRANSACTION_RECEIVER_SIZE]byte
	String() string
}
