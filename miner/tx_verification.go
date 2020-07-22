package miner

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"github.com/julwil/bazo-miner/crypto"
	"github.com/julwil/bazo-miner/protocol"
	"github.com/julwil/bazo-miner/storage"
	"math/big"
	"reflect"
)

//We can't use polymorphism, e.g. we can't use tx.verify() because the Transaction interface doesn't declare
//the verify method. This is because verification depends on the State (e.g., dynamic properties), which
//should only be of concern to the miner, not to the protocol package. However, this has the disadvantage
//that we have to do case distinction here.
func verify(tx protocol.Transaction) bool {
	switch tx.(type) {
	case *protocol.FundsTx:
		return verifyFundsTx(tx.(*protocol.FundsTx))

	case *protocol.AccTx:
		return verifyAccTx(tx.(*protocol.AccTx))

	case *protocol.ConfigTx:
		return verifyConfigTx(tx.(*protocol.ConfigTx))

	case *protocol.StakeTx:
		return verifyStakeTx(tx.(*protocol.StakeTx))

	case *protocol.AggTx:
		return verifyAggTx(tx.(*protocol.AggTx))

	case *protocol.UpdateTx:
		return verifyUpdateTx(tx.(*protocol.UpdateTx))

	default: // In case tx is nil or we encounter an unhandled transaction type
		return false
	}
}

func verifyFundsTx(tx *protocol.FundsTx) bool {

	logger.Printf("Verifying: \n%s", tx.String())

	//fundsTx only makes sense if amount > 0
	if tx.Amount == 0 || tx.Amount > MAX_MONEY {
		logger.Printf("Invalid transaction amount: %v\n", tx.Amount)
		return false
	}

	//Check if accounts are present in the actual state
	accFrom := storage.State[tx.From]
	accTo := storage.State[tx.To]

	logger.Printf("\nFrom:\n%s", accFrom.String())
	logger.Printf("\nTo:\n%s", accTo.String())

	if accFrom == nil || accTo == nil {
		logger.Printf("Account non existent. From: %v\nTo: %v\n", accFrom, accTo)
		return false
	}

	// Check if From & To are different accounts
	if reflect.DeepEqual(accFrom, accTo) {
		logger.Printf("From account equals To account. From: %v\nTo: %v\n", accFrom, accTo)
	}

	txHash := tx.Hash()

	var validSig1, validSig2 bool

	// Validate Sig1
	validSig1 = IsSigned(txHash, tx.Sig1, accFrom.Address)

	// If no Sig2 is specified, we don't validate it.
	if tx.Sig2 == [64]byte{} {
		validSig2 = true
	}

	// Validate Sig2
	if tx.Sig2 != [64]byte{} {

		validSig2 = IsSigned(txHash, tx.Sig2, crypto.GetAddressFromPubKey(multisigPubKey))
	}

	return validSig1 && validSig2
}

func verifyAccTx(tx *protocol.AccTx) bool {
	for _, rootAcc := range storage.RootKeys {
		signature := tx.Sig
		address := rootAcc.Address
		txHash := tx.Hash()

		if IsSigned(txHash, signature, address) {
			return true
		}
	}

	logger.Printf("No valid root account.")
	return false
}

func verifyConfigTx(tx *protocol.ConfigTx) bool {
	//account creation can only be done with a valid priv/pub key which is hard-coded

	for _, rootAcc := range storage.RootKeys {
		signature := tx.Sig
		address := rootAcc.Address
		txHash := tx.Hash()
		if IsSigned(txHash, signature, address) {
			return true
		}
	}

	logger.Printf("No valid root account.")

	return false
}

func verifyStakeTx(tx *protocol.StakeTx) bool {
	//Check if account is present in the actual state
	accFrom := storage.State[tx.Account]

	//Account non existent
	if accFrom == nil {
		logger.Println("Account does not exist.")

		return false
	}

	accFromHash := protocol.SerializeHashContent(accFrom.Address)
	tx.Account = accFromHash
	txHash := tx.Hash()

	return IsSigned(txHash, tx.Sig, accFrom.Address)
}

//TODO Update this function
func verifyAggTx(tx *protocol.AggTx) bool {
	//Check if accounts are existent
	//accSender, err := storage.GetAccount(tx.From)
	//if tx.From //!= protocol.SerializeHashContent(accSender.Address) || tx.To == nil || err != nil {
	//	logger.Printf("Account non existent. From: %v\nTo: %v\n%v", tx.From, tx.To, err)
	//	return false
	//}

	return true
}

func verifyUpdateTx(tx *protocol.UpdateTx) bool {

	// First we make sure that the account of the tx issuer exists.
	issuerAccount := storage.State[tx.Issuer]
	if issuerAccount == nil {
		logger.Printf("Account of tx issuer does not exist: %v", tx.Issuer)
		return false
	}

	// Next we check if the tx to update actually exists.
	var txToDelete protocol.Transaction

	switch true {
	case storage.ReadOpenTx(tx.TxToUpdateHash) != nil:
		txToDelete = storage.ReadOpenTx(tx.TxToUpdateHash)

	case storage.ReadClosedTx(tx.TxToUpdateHash) != nil:
		txToDelete = storage.ReadClosedTx(tx.TxToUpdateHash)

	default: // If we don't find the tx to update in the storage, we also can't update it.
		logger.Printf("Can't find TxToDelete: %x", tx.TxToUpdateHash)
		return false
	}

	txHash := tx.Hash()
	isTxSigned := IsSigned(txHash, tx.Sig, issuerAccount.Address)

	if !isTxSigned {
		logger.Printf("Tx: %x not signed correctly", txHash)
		return false
	}

	// Lastly we check if the issuer of the update-tx also signed the tx to update.
	// This makes sure that you only update your own txs.
	// Get the hash
	txToUpdateHash := txToDelete.Hash()
	var txToUpdateSig [64]byte

	// Now we validate the txToDelete and retrieve its signature.
	switch txToDelete.(type) {
	case *protocol.FundsTx:
		txToUpdateSig = txToDelete.(*protocol.FundsTx).Sig1

	case *protocol.AccTx:
		txToUpdateSig = txToDelete.(*protocol.AccTx).Sig

	case *protocol.UpdateTx:
		txToUpdateSig = txToDelete.(*protocol.UpdateTx).Sig

	case *protocol.ConfigTx:
		logger.Printf("\nCan't update config tx")
		return false

	case *protocol.StakeTx:
		logger.Printf("\nCan't update stake tx")
		return false

	case *protocol.AggTx:
		logger.Printf("\nCan't update aggregate tx")
		return false

	default: // In case we can't cast the tx to a known type, abort
		return false
	}

	isAuthorized := IsSigned(txToUpdateHash, txToUpdateSig, issuerAccount.Address)

	if !isAuthorized {
		logger.Printf("\nISSUER NOT ALLOWED TO UPDATE TX."+
			"\nTxToUpdate was not signed by Issuer. (You can only update your own tx)"+
			"\nIssuer: %x"+
			"\nTxToUpdate: %x"+
			"\nAbort update.", tx.Issuer, txToUpdateHash)

		return false
	}

	return true
}

//Returns true if id is in the list of possible ids and rational value for payload parameter.
//Some values just don't make any sense and have to be restricted accordingly
func parameterBoundsChecking(id uint8, payload uint64) bool {
	switch id {
	case protocol.BLOCK_SIZE_ID:
		if payload >= protocol.MIN_BLOCK_SIZE && payload <= protocol.MAX_BLOCK_SIZE {
			return true
		}
	case protocol.DIFF_INTERVAL_ID:
		if payload >= protocol.MIN_DIFF_INTERVAL && payload <= protocol.MAX_DIFF_INTERVAL {
			return true
		}
	case protocol.FEE_MINIMUM_ID:
		if payload >= protocol.MIN_FEE_MINIMUM && payload <= protocol.MAX_FEE_MINIMUM {
			return true
		}
	case protocol.BLOCK_INTERVAL_ID:
		if payload >= protocol.MIN_BLOCK_INTERVAL && payload <= protocol.MAX_BLOCK_INTERVAL {
			return true
		}
	case protocol.BLOCK_REWARD_ID:
		if payload >= protocol.MIN_BLOCK_REWARD && payload <= protocol.MAX_BLOCK_REWARD {
			return true
		}
	case protocol.STAKING_MINIMUM_ID:
		if payload >= protocol.MIN_STAKING_MINIMUM && payload <= protocol.MAX_STAKING_MINIMUM {
			return true
		}
	case protocol.WAITING_MINIMUM_ID:
		if payload >= protocol.MIN_WAITING_TIME && payload <= protocol.MAX_WAITING_TIME {
			return true
		}
	case protocol.ACCEPTANCE_TIME_DIFF_ID:
		if payload >= protocol.MIN_ACCEPTANCE_TIME_DIFF && payload <= protocol.MAX_ACCEPTANCE_TIME_DIFF {
			return true
		}
	case protocol.SLASHING_WINDOW_SIZE_ID:
		if payload >= protocol.MIN_SLASHING_WINDOW_SIZE && payload <= protocol.MAX_SLASHING_WINDOW_SIZE {
			return true
		}
	case protocol.SLASHING_REWARD_ID:
		if payload >= protocol.MIN_SLASHING_REWARD && payload <= protocol.MAX_SLASHING_REWARD {
			return true
		}
	}

	return false
}

// Checks if the hash is signed by the provided signature and address
func IsSigned(hash [32]byte, signature [64]byte, address [64]byte) bool {
	pub1, pub2 := new(big.Int), new(big.Int)
	r, s := new(big.Int), new(big.Int)

	r.SetBytes(signature[:32])
	s.SetBytes(signature[32:])
	pub1.SetBytes(address[:32])
	pub2.SetBytes(address[32:])
	publicKey := ecdsa.PublicKey{elliptic.P256(), pub1, pub2}

	//fmt.Printf("\nAddress: %x\nSignature: %x\nHash:%x\n", address, signature, hash)

	isSigned := ecdsa.Verify(&publicKey, hash[:], r, s)
	if !isSigned {
		fmt.Printf("Could not verfiy signature.")
	}

	return isSigned
}
