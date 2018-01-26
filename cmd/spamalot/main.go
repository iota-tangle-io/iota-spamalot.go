/*
MIT License

Copyright (c) 2018 iota-tangle.io

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cwarner818/giota"
	flag "github.com/ogier/pflag"
)

var (
	mwm   *int64 = flag.Int64("mwm", 14, "minimum weight magnitude")
	depth *int64 = flag.Int64("depth", giota.Depth, "whatever depth is")

	destAddress *string = flag.String("dest",
		"SPPRLTTIVYUONPOPQSWGCPMZWDOMQGWFUEPKUQIVUKROCHRNCR9MXNGNQSAGLKUDX9MZQWCPFJQS9DWAY", "address to send to")

	tag    *string = flag.String("tag", "999SPAMALOT", "transaction tag")
	msg    *string = flag.String("msg", "GOSPAMMER9VERSION9ONE9ONE", "transaction message")
	server *string = flag.String("node", "http://localhost:14265", "remote node to connect to")

	filterTrunk *bool = flag.Bool("trunk", false,
		"do not send a transaction with our own transaction as a trunk")

	filterBranch *bool = flag.Bool("branch", false,
		"do not send a transaction with our own transaction as a branch")

	filterBoth *bool = flag.Bool("both", false,
		"do not send a transaction with our own transaction as a branch and a trunk")

	badTrunk, badBranch, badBoth int

	remotePow *bool = flag.Bool("pow", false,
		"if set, do PoW calculation on remote node via API")
)

func main() {
	flag.Parse()
	seed := giota.NewSeed()

	recipientT, err := giota.ToAddress(*destAddress)
	if err != nil {
		panic(err)
	}
	ttag, err := giota.ToTrytes(*tag)
	if err != nil {
		panic(err)
	}
	tmsg, err := giota.ToTrytes(*msg)
	if err != nil {
		panic(err)
	}
	trs := []giota.Transfer{
		giota.Transfer{
			Address: recipientT,
			Value:   0,
			Tag:     ttag,
			Message: tmsg,
		},
	}

	var bdl giota.Bundle

	fmt.Printf("using IRI server: %s\n", *server)

	api := giota.NewAPI(*server, nil)
	var pow giota.PowFunc

	if !*remotePow {
		var name string
		name, pow = giota.GetBestPoW()
		fmt.Fprintf(os.Stderr, "using PoW:%s\n", name)
	} else {
		fmt.Fprintf(os.Stderr, "using PoW:attachToTangle\n")
	}

	var txnCount float64
	var totalTime float64
	var good, bad int
	for {
		start := time.Now()
		txnCount++
		bdl, err = giota.PrepareTransfers(api, seed, trs, nil, "", 2)
		if err != nil {
			log.Println("Error preparing transfer:", err)
			bad++
		} else {
			err = SendTrytes(api, *depth, []giota.Transaction(bdl), *mwm, pow)
			if err != nil {
				log.Println("Error sending transaction:", err)
				bad++
			}
		}

		if err == nil {
			good++
			log.Println("SENT:", bdl.Hash())
		}

		dur := time.Since(start)
		totalTime += dur.Seconds()
		tps := txnCount / totalTime
		log.Printf("%.2f TPS -- %.0f%% success", tps,
			100*(float64(good)/(float64(good)+float64(bad))))

		log.Printf("Duration: %s Count: %.0f Bad Trunk: %d Bad Branch: %d Both: %d",
			dur.String(), txnCount, badTrunk, badBranch, badBoth)

	}
}

func SendTrytes(api *giota.API, depth int64, trytes []giota.Transaction, mwm int64, pow giota.PowFunc) error {
	tra, err := api.GetTransactionsToApprove(depth)
	if err != nil {
		return err
	}

	txns, err := api.GetTrytes([]giota.Trytes{
		tra.TrunkTransaction,
		tra.BranchTransaction,
	})

	if err != nil {
		return err
	}

	tTag := string(txns.Trytes[0].Tag)
	bTag := string(txns.Trytes[1].Tag)
	if *filterBoth && (tTag == bTag) && bTag == "999SPAMALOT9999999999999999" {
		badBoth++
		return errors.New("Trunk and branch tag is ours")
	} else if *filterTrunk && tTag == "999SPAMALOT9999999999999999" {
		badTrunk++
		return errors.New("Trunk tag is ours")
	} else if *filterBranch && bTag == "999SPAMALOT9999999999999999" {
		badBranch++
		return errors.New("Branch tag is ours")
	}
	switch {
	case *remotePow || pow == nil:
		at := giota.AttachToTangleRequest{
			TrunkTransaction:   tra.TrunkTransaction,
			BranchTransaction:  tra.BranchTransaction,
			MinWeightMagnitude: mwm,
			Trytes:             trytes,
		}

		// attach to tangle - do pow
		attached, err := api.AttachToTangle(&at)
		if err != nil {
			return err
		}

		trytes = attached.Trytes
	default:
		err := doPow(tra, depth, trytes, mwm, pow)
		if err != nil {
			return err
		}
	}

	// Broadcast and store tx
	return api.BroadcastTransactions(trytes)
}
func doPow(tra *giota.GetTransactionsToApproveResponse, depth int64, trytes []giota.Transaction, mwm int64, pow giota.PowFunc) error {
	var prev giota.Trytes
	var err error
	for i := len(trytes) - 1; i >= 0; i-- {
		switch {
		case i == len(trytes)-1:
			trytes[i].TrunkTransaction = tra.TrunkTransaction
			trytes[i].BranchTransaction = tra.BranchTransaction
		default:
			trytes[i].TrunkTransaction = prev
			trytes[i].BranchTransaction = tra.TrunkTransaction
		}

		trytes[i].Nonce, err = pow(trytes[i].Trytes(), int(mwm))
		if err != nil {
			return err
		}

		prev = trytes[i].Hash()
	}
	return nil
}
