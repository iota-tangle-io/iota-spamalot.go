package spamalot

import (
	"errors"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/cwarner818/giota"
)

type worker struct {
	node        Node
	api         *giota.API
	spammer     *Spammer
	stopSignal  chan struct{}
	sendMetrics bool
}

// retrieves tips from the given node and puts them into the tips channel
func (w worker) getNonZeroTips(tipsChan chan Tips, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-w.stopSignal:
			return
		default:
			tips, err := w.api.GetTips()
			if err != nil {
				w.spammer.logIfVerbose("GetTips error", err)
				continue
			}

			// Loop through returned tips and get a random txn
			// if txn value is zero, get a new one
			var txn *giota.Transaction
			var txnHash giota.Trytes
			for {
				if len(tips.Hashes) == 0 {
					break
				}

				r := rand.Intn(len(tips.Hashes))
				txns, err := w.api.GetTrytes([]giota.Trytes{tips.Hashes[r]})
				if err != nil {
					w.spammer.logIfVerbose("GetTrytes error:", err)
					continue
				}

				txn = &txns.Trytes[0]
				if txn.Value == 0 {
					tips.Hashes = append(tips.Hashes[:r],
						tips.Hashes[r+1:]...)
					continue
				}
				txnHash = tips.Hashes[r]
				break
			}

			if txn == nil {
				continue
			}

			w.spammer.logIfVerbose("Got tips from", w.node.URL)

			nodeInfo, err := w.api.GetNodeInfo()
			if err != nil {
				w.spammer.logIfVerbose("GetNodeInfo error:", err)
				continue
			}
			txns, err := w.api.GetTrytes([]giota.Trytes{nodeInfo.LatestMilestone})
			if err != nil {
				w.spammer.logIfVerbose("GetTrytes error:", err)
				continue
			}

			milestone := txns.Trytes[0]

			tip := Tips{
				Trunk:      milestone,
				TrunkHash:  nodeInfo.LatestMilestone,
				Branch:     *txn,
				BranchHash: txnHash,
			}

			select {
			case <-w.stopSignal:
				return
			default:
				select {
				case <-w.stopSignal:
					return
				case tipsChan <- tip:
				}
			}
		}
	}
}

// retrieve the tips from the database or fetch them via api and set tips to
// their completed transactions
func (w worker) loadOrFetchTips(tips *Tips) error {
	//Query the database to see if we have the transactions cached
	storedTxns, err := w.spammer.db.GetTransactions([]giota.Trytes{
		tips.TrunkHash,
		tips.BranchHash,
	})

	if err != nil {
		return errors.New("Error loading stored tips: " + err.Error())
	}

	var fetchTrunk, fetchBranch bool
	fetchTxns := make([]giota.Trytes, 0)

	if storedTxns[0] == nil {
		fetchTxns = append(fetchTxns, tips.TrunkHash)
		fetchTrunk = true
		w.spammer.metrics.addMetric(INC_NEW_CACHED_TX, nil)
		//log.Println("Fetching trunk:", tips.TrunkHash)

	} else {
		w.spammer.metrics.addMetric(INC_GET_CACHED_TX, nil)
		//log.Println("Loaded trunk:", tips.TrunkHash)
	}

	if storedTxns[1] == nil {
		fetchTxns = append(fetchTxns, tips.BranchHash)
		fetchBranch = true
		w.spammer.metrics.addMetric(INC_NEW_CACHED_TX, nil)
		//log.Println("Fetching branch:", tips.BranchHash)

	} else {
		w.spammer.metrics.addMetric(INC_GET_CACHED_TX, nil)
		//log.Println("Loaded branch:", tips.BranchHash)
	}

	txns, err := w.api.GetTrytes(fetchTxns)
	if err != nil {
		return errors.New("Error fetching new tips: " + err.Error())
	}

	if fetchTrunk && fetchBranch {
		tips.Trunk = txns.Trytes[0]
		tips.Branch = txns.Trytes[1]
		w.spammer.db.StoreTransactions(txns.Trytes)
	} else if fetchTrunk {
		tips.Trunk = txns.Trytes[0]
		w.spammer.db.StoreTransactions(txns.Trytes)
	} else if fetchBranch {
		tips.Branch = txns.Trytes[0]
		w.spammer.db.StoreTransactions(txns.Trytes)
	}

	return nil

}

// retrieves tips from the given node and puts them into the tips channel
func (w worker) getTxnsToApprove(tipsChan chan Tips, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-w.stopSignal:
			return
		default:
			tips, err := w.api.GetTransactionsToApprove(w.spammer.depth, giota.DefaultNumberOfWalks, "")
			if err != nil {
				w.spammer.logIfVerbose("GetTransactionsToApprove error", err)
				continue
			}

			tip := &Tips{
				TrunkHash:  tips.TrunkTransaction,
				BranchHash: tips.BranchTransaction,
			}

			// Retrieved cached transactions from the database
			// or fetch them via the IRI API and store them
			err = w.loadOrFetchTips(tip)
			if err != nil {
				w.spammer.logIfVerbose("loadOrFetchTips error", err)
				continue
			}
			select {
			case <-w.stopSignal:
				return
			default:
				select {
				case <-w.stopSignal:
					return
				case tipsChan <- *tip:
				}
			}

		}
	}
}

// receives prepared txs and attaches them via remote node or local PoW onto the tangle
func (w worker) spam(txnChan <-chan Transaction, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-w.stopSignal:
			return
		default:
			select {
			case <-w.stopSignal:
				return
				// read next tx to processes
			case txn, ok := <-txnChan:
				if !ok {
					return
				}

				switch {
				case !w.spammer.localPoW && w.node.AttachToTangle:

					w.spammer.logIfVerbose("attaching to tangle")

					at := giota.AttachToTangleRequest{
						TrunkTransaction:   txn.Trunk,
						BranchTransaction:  txn.Branch,
						MinWeightMagnitude: w.spammer.mwm,
						Trytes:             txn.Transactions,
					}

					attached, err := w.api.AttachToTangle(&at)
					if err != nil {
						w.spammer.metrics.addMetric(INC_FAILED_TX, nil)
						log.Println("Error attaching to tangle:", err)
						continue
					}

					txn.Transactions = attached.Trytes
				default:

					// lock so only one worker is doing PoW at a time
					w.spammer.powMu.Lock()
					w.spammer.logIfVerbose("doing PoW")

					err := doPow(&txn, w.spammer.depth, txn.Transactions, w.spammer.mwm, w.spammer.pow)
					if err != nil {
						w.spammer.metrics.addMetric(INC_FAILED_TX, nil)
						log.Println("Error doing PoW:", err)
						w.spammer.powMu.Unlock()
						continue
					}
					w.spammer.powMu.Unlock()
				}

				err := w.api.BroadcastTransactions(txn.Transactions)

				w.spammer.RLock()
				defer w.spammer.RUnlock()
				if err != nil {
					w.spammer.metrics.addMetric(INC_FAILED_TX, nil)
					log.Println(w.node, "ERROR:", err)
					continue
				}

				// this will auto print metrics to console
				w.spammer.metrics.addMetric(INC_SUCCESSFUL_TX, txandnode{txn, w.node})

				// wait the cooldown before accepting a new TX
				if w.spammer.cooldown > 0 {
					select {
					case <-w.stopSignal:
						return
					case <-time.After(w.spammer.cooldown):
					}
				}
			}
		}
	}
}
