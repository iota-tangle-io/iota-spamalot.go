package spamalot

import (
	"fmt"
	"log"
	"time"

	"github.com/coreos/bbolt"
	"github.com/cwarner818/giota"
)

type Database struct {
	*bolt.DB
	runKey string
}

func NewDatabase(db *bolt.DB) *Database {
	return &Database{
		DB: db,
	}
}

func (s *Database) dbNewRun(runKey string) {
	// Check to make sure we are using a database
	if s.DB == nil {
		return
	}

	s.runKey = runKey

	s.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("runs"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		runBucket, err := b.CreateBucketIfNotExists([]byte(runKey))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		b, err = runBucket.CreateBucketIfNotExists([]byte("transactions"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		b, err = runBucket.CreateBucketIfNotExists([]byte("logs"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
}

func (s *Database) dbLog(msg string) {
	// Check to make sure we are using a database
	if s.DB == nil {
		return
	}
	s.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("runs"))
		b = b.Bucket([]byte(s.runKey))
		b = b.Bucket([]byte("logs"))
		return b.Put([]byte(time.Now().Format((time.RFC3339))), []byte(msg))
	})
}
func (s *Database) dbLogTransactions(txns []giota.Transaction) {
	// Check to make sure we are using a database
	if s.DB == nil {
		return
	}

	s.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("runs"))
		b = b.Bucket([]byte(s.runKey))
		b = b.Bucket([]byte("transactions"))

		for _, txn := range txns {
			json, err := txn.MarshalJSON()
			if err != nil {
				log.Println("ERROR JSON:", err)
				return err
			}

			err = b.Put([]byte(txn.Hash()), json)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
