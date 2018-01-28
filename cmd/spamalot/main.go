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
	"log"
	"time"

	"github.com/cwarner818/giota"
	spamalot "github.com/iota-tangle-io/iota-spamalot.go"
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

	badTrunk, badBranch, badBoth int

	remotePow *bool = flag.Bool("pow", false,
		"if set, do PoW calculation on remote node via API")
)

func main() {
	flag.Parse()

	s, err := spamalot.New(*server,
		spamalot.WithMWM(*mwm),
		spamalot.WithDepth(*depth),
		spamalot.ToAddress(*destAddress),
		spamalot.WithTag(*tag),
		spamalot.WithMessage(*msg),
		spamalot.FilterTrunk(*filterTrunk),
		spamalot.FilterBranch(*filterBranch),
		spamalot.WithRemotePoW(*remotePow),
	)

	if err != nil {
		log.Println(err)
		return
	}

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
	log.Println("IOTΛ Spamalot starting")

	log.Println("Using IRI node:", *server)

	api := giota.NewAPI(*server, nil)
	var pow giota.PowFunc

	powName := "attachToTangle"
	if !*remotePow {
		var name string
		name, pow = giota.GetBestPoW()
		powName = name
	}
	log.Println("Using PoW:", powName)

	var txnCount float64
	//var totalTime float64
	var good, bad int
	start := time.Now()
	for {
		txnCount++
		bdl, err = giota.PrepareTransfers(api, seed, trs, nil, "", 2)
		if err != nil {
			log.Println("Error preparing transfer:", err)
			bad++
		} else {
			err = s.SendTrytes(api, *depth, []giota.Transaction(bdl), *mwm, pow)
			if err != nil {
				log.Println("Error sending transaction:", err)
				bad++
			}
		}

		if err == nil {
			good++
			log.Println("http://thetangle.org/bundle/" + bdl.Hash())
		}

		dur := time.Since(start)
		//totalTime += dur.Seconds()
		tps := float64(good) / dur.Seconds()
		log.Printf("%.2f TPS -- %.0f%% success", tps,
			100*(float64(good)/(float64(good)+float64(bad))))

		log.Printf("Duration: %s Count: %.0f Bad Trunk: %d Bad Branch: %d Both: %d",
			dur.String(), txnCount, badTrunk, badBranch, badBoth)

	}
}
