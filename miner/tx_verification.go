package miner

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"
	"reflect"

	"github.com/julwil/bazo-miner/protocol"
	"github.com/julwil/bazo-miner/storage"
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

	case *protocol.DeleteTx:
		return verifyDeleteTx(tx.(*protocol.DeleteTx))

	default: // In case tx is nil or we encounter an unhandled transaction type
		return false
	}
}

func verifyFundsTx(tx *protocol.FundsTx) bool {

	//fundsTx only makes sense if amount > 0
	if tx.Amount == 0 || tx.Amount > MAX_MONEY {
		logger.Printf("Invalid transaction amount: %v\n", tx.Amount)
		return false
	}

	//Check if accounts are present in the actual state
	accFrom := storage.State[tx.From]
	accTo := storage.State[tx.To]

	//Accounts non existent
	if accFrom == nil || accTo == nil {
		logger.Printf("Account non existent. From: %v\nTo: %v\n", accFrom, accTo)
		return false
	}

	accFromHash := protocol.SerializeHashContent(accFrom.Address)
	accToHash := protocol.SerializeHashContent(accTo.Address)
	tx.From = accFromHash
	tx.To = accToHash
	txHash := tx.Hash()

	var validSig1, validSig2 bool

	if isSigned(txHash, tx.Sig1, accFrom.Address) && !reflect.DeepEqual(accFrom, accTo) {
		tx.From = accFromHash
		tx.To = accToHash
		validSig1 = true
	} else {
		logger.Printf("Sig1 invalid. FromHash: %x\nToHash: %x\n", accFromHash[0:8], accToHash[0:8])
		logger.Printf("Invalid TX:\n%s", tx.String())
		return false
	}

	r, s := new(big.Int), new(big.Int)
	r.SetBytes(tx.Sig2[:32])
	s.SetBytes(tx.Sig2[32:])

	if ecdsa.Verify(multisigPubKey, txHash[:], r, s) {
		validSig2 = true
	} else {
		logger.Printf("Sig2 invalid. FromHash: %x\nToHash: %x\n", accFromHash[0:8], accToHash[0:8])
		return false
	}

	return validSig1 && validSig2
}

func verifyAccTx(tx *protocol.AccTx) bool {
	for _, rootAcc := range storage.RootKeys {
		signature := tx.Sig
		address := rootAcc.Address
		txHash := tx.Hash()

		if isSigned(txHash, signature, address) {
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
		if isSigned(txHash, signature, address) {
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

	return isSigned(txHash, tx.Sig, accFrom.Address)
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

func verifyDeleteTx(tx *protocol.DeleteTx) bool {

	// First we make sure that the account of the tx issuer exists.
	issuerAccount := storage.State[tx.Issuer]
	if issuerAccount == nil {
		logger.Printf("Account of tx issuer does not exist: %v", tx.Issuer)
		return false
	}

	// Next we check if the tx to delete actually exists.
	var txToDelete protocol.Transaction

	switch true {
	case storage.ReadOpenTx(tx.TxToDeleteHash) != nil:
		txToDelete = storage.ReadOpenTx(tx.TxToDeleteHash)

	case storage.ReadClosedTx(tx.TxToDeleteHash) != nil:
		txToDelete = storage.ReadClosedTx(tx.TxToDeleteHash)

	default: // If we don't find the tx to delete in the storage, we also can't delete it.
		logger.Printf("Can't find TxToDelete: %x", tx.TxToDeleteHash)
		return false
	}

	txHash := tx.Hash()
	isTxSigned := isSigned(txHash, tx.Sig, issuerAccount.Address)

	if !isTxSigned {
		logger.Printf("Tx: %x not signed correctly", txHash)
		return false
	}

	// Lastly we check if the issuer of the delete-tx also signed the tx to delete.
	// This makes sure that you only delete your own txs.
	// Get the hash
	txToDeleteHash := txToDelete.Hash()
	var txToDeleteSig [64]byte

	// Now we validate the txToDelete and retrieve its signature.
	switch txToDelete.(type) {
	case *protocol.FundsTx:
		txToDeleteSig = txToDelete.(*protocol.FundsTx).Sig1

	case *protocol.AccTx:
		logger.Printf("\nCan't delete account tx")
		return false

	case *protocol.ConfigTx:
		logger.Printf("\nCan't delete config tx")
		return false

	case *protocol.StakeTx:
		logger.Printf("\nCan't delete stake tx")
		return false

	case *protocol.AggTx:
		logger.Printf("\nCan't delete aggregate tx")
		return false

	case *protocol.DeleteTx:
		logger.Printf("\nCan't delete delete tx")
		return false

	default: // In case we can't cast the tx to a known type, abort
		return false
	}

	isAuthorized := isSigned(txToDeleteHash, txToDeleteSig, issuerAccount.Address)

	if !isAuthorized {
		logger.Printf("\nISSUER NOT ALLOWED TO DELETE TX."+
			"\nTxToDelete was not signed by Issuer. (You can only delete your own tx)"+
			"\nIssuer: %x"+
			"\nTxToDelete: %x"+
			"\nAbort deletion.", tx.Issuer, txToDeleteHash)
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
func isSigned(hash [32]byte, signature [64]byte, address [64]byte) bool {
	pub1, pub2 := new(big.Int), new(big.Int)
	r, s := new(big.Int), new(big.Int)

	r.SetBytes(signature[:32])
	s.SetBytes(signature[32:])
	pub1.SetBytes(address[:32])
	pub2.SetBytes(address[32:])
	publicKey := ecdsa.PublicKey{elliptic.P256(), pub1, pub2}

	isSigned := ecdsa.Verify(&publicKey, hash[:], r, s)
	if !isSigned {
		logger.Printf("Could not verfiy signature.")
	}

	return isSigned
}
