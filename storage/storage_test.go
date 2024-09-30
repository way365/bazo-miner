package storage

import (
	"encoding/hex"
	"fmt"
	"github.com/boltdb/bolt"
	"testing"
	"time"
)

func TestReadWriteUpdateTx(t *testing.T) {
	db, err := bolt.Open("StoreA.db", 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		println(err)
	}
	bytes, err := hex.DecodeString("5081ec5b666bb3e74895b15f496b7e1c3ae046ae2d5545bd1f5a37ee8f718d5a")
	var blockHash [32]byte
	db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("blocksbytxhash"))
		copy(blockHash[:], bucket.Get(bytes))
		return nil
	})
	fmt.Printf("block hash: %x\n", blockHash)
}
