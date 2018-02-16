package spamalot

import (
	"fmt"
	"log"
	"time"

	"github.com/coreos/bbolt"
	"github.com/cwarner818/giota"
)

// time.RFC3339Nano drops trailing zeros which breaks the ability to use this
// so we create our own
const rfc3339nano = "2006-01-02T15:04:05.000000000Z07:00"

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
		_, err := tx.CreateBucketIfNotExists([]byte("transactions"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		runs, err := tx.CreateBucketIfNotExists([]byte("runs"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		runBucket, err := runs.CreateBucketIfNotExists([]byte(runKey))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		_, err = runBucket.CreateBucketIfNotExists([]byte("sent"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}

		_, err = runBucket.CreateBucketIfNotExists([]byte("logs"))
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
		return b.Put([]byte(time.Now().Format(rfc3339nano)), []byte(msg))
	})
}

func (s *Database) GetTransactions(txns []giota.Trytes) ([]*giota.Transaction, error) {
	// Check to make sure we are using a database
	if s.DB == nil {
		return nil, nil
	}
	output := make([]*giota.Transaction, len(txns))
	err := s.View(func(tx *bolt.Tx) error {
		txnsBucket := tx.Bucket([]byte("transactions"))
		if txnsBucket == nil {
			log.Fatal("NIL BUCKET")
		}

		for i, txn := range txns {
			v := txnsBucket.Get([]byte(txn))
			if v == nil {
				output[i] = nil
				continue
			}

			t := &giota.Transaction{}

			err := t.UnmarshalJSON(v)
			if err != nil {
				return err
			}
			output[i] = t
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return output, nil
}
func (s *Database) LogTips(txns []giota.Transaction) {
	// Check to make sure we are using a database
	if s.DB == nil {
		return
	}
	s.Update(func(tx *bolt.Tx) error {

		// Store hash => transaction
		txnsBucket := tx.Bucket([]byte("transactions"))
		if txnsBucket == nil {
			log.Fatal("NIL BUCKET")
		}
		//b = transactions.Bucket([]byte("transactions"))
		for _, txn := range txns {
			json, err := txn.MarshalJSON()
			if err != nil {
				log.Println("ERROR JSON:", err)
				return err
			}

			err = txnsBucket.Put([]byte(txn.Hash()), json)
			if err != nil {
				return err
			}
		}

		return nil
	})
}
func (s *Database) StoreTransactions(txns []giota.Transaction) {
	// Check to make sure we are using a database
	if s.DB == nil {
		return
	}
	s.Update(func(tx *bolt.Tx) error {

		// Store hash => transaction
		txnsBucket := tx.Bucket([]byte("transactions"))
		if txnsBucket == nil {
			log.Fatal("NIL BUCKET")
		}
		//b = transactions.Bucket([]byte("transactions"))
		for _, txn := range txns {
			json, err := txn.MarshalJSON()
			if err != nil {
				log.Println("ERROR JSON:", err)
				return err
			}

			//log.Println("Storing:", txn.Hash())
			err = txnsBucket.Put([]byte(txn.Hash()), json)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Database) LogSentTransactions(txns []giota.Transaction) {
	// Check to make sure we are using a database
	if s.DB == nil {
		return
	}
	/*
		Database schema:

		transactions
		  hash => full transaction trytes
		runs // contains all runs of the spammer
		  <timestamp run started>
		    config
		      nodes // nodes spammer used for run
			host:port
			  tips // tip txns received from node
			    timestamp => tips{trunk, branch}
			  transactions // txns sent to node
			    timestamp => hash
			log
			  timestamp => message
		    sent // complete sent txns, globally
		      timestamp => hash

		    logs
		      timestamp => log message
	*/

	s.Update(func(tx *bolt.Tx) error {

		// Store hash => transaction
		txnsBucket := tx.Bucket([]byte("transactions"))
		if txnsBucket == nil {
			log.Fatal("NIL BUCKET")
		}
		//b = transactions.Bucket([]byte("transactions"))
		for _, txn := range txns {
			json, err := txn.MarshalJSON()
			if err != nil {
				log.Println("ERROR JSON:", err)
				return err
			}

			err = txnsBucket.Put([]byte(txn.Hash()), json)
			if err != nil {
				return err
			}
		}

		b := tx.Bucket([]byte("runs"))
		runBucket := b.Bucket([]byte(s.runKey))
		// Store timestamp => hash
		//thisRun := runBucket.Bucket([]byte("timestamp"))
		b = runBucket.Bucket([]byte("sent"))
		if b == nil {
			log.Fatal("nil bucket")
		}
		for _, txn := range txns {
			err := b.Put([]byte(
				txn.Timestamp.Format(rfc3339nano)),
				[]byte(txn.Hash()))
			if err != nil {
				return err
			}
		}

		return nil
	})
}
