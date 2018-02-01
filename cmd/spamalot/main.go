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

	"github.com/CWarner818/giota"
	spamalot "github.com/iota-tangle-io/iota-spamalot.go"
	flag "github.com/spf13/pflag"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	nodeAddr *string = flag.String("node", "http://localhost:14625", "remote IRI node")
	mwm   *int64 = flag.Int64("mwm", 14, "minimum weight magnitude")
	depth *int64 = flag.Int64("depth", giota.Depth, "the milestone depth used by the MCMC")
	timeout *int64 = flag.Int64("timeout", 0, "how long to let the spammer run in seconds (if not specified infinite)")
	securityLvl *int64 = flag.Int64("security-lvl", 2, "the security lvl used for generating addresses")

	destAddress *string = flag.String("dest",
		"SPPRLTTIVYUONPOPQSWGCPMZWDOMQGWFUEPKUQIVUKROCHRNCR9MXNGNQSAGLKUDX9MZQWCPFJQS9DWAY", "address to send to")

	tag *string = flag.String("tag", "999SPAMALOT", "transaction tag")
	msg *string = flag.String("msg", "GOSPAMMER9VERSION9ONE9THREE", "transaction message")
	//nodes *[]string = flag.StringSlice("node", []string{"http://localhost:14265"}, "remote node to connect to")

	remotePoW *bool = flag.Bool("remote-pow", false,
		"whether to let the remote IRI node do the PoW")

	filterTrunk *bool = flag.Bool("trunk", false,
		"do not send a transaction with our own transaction as a trunk")

	filterBranch *bool = flag.Bool("branch", false,
		"do not send a transaction with our own transaction as a branch")

	filterMilestone *bool = flag.Bool("milestone", false,
		"do not send a transaction with a milestone as a trunk or branch")
	remotePow *bool = flag.Bool("pow", false,
		"if set, do PoW calculation on remote node via API")
)

func main() {
	flag.Parse()

	var pow giota.PowFunc
	var powName string
	if !*remotePow {
		powName, pow = giota.GetBestPoW()
		log.Println("Using PoW:", powName)

	}
	s, err := spamalot.New(
		spamalot.WithNode(*nodeAddr, *remotePoW),
		spamalot.WithMWM(*mwm),
		spamalot.WithDepth(*depth),
		spamalot.ToAddress(*destAddress),
		spamalot.WithTag(*tag),
		spamalot.WithMessage(*msg),
		spamalot.FilterTrunk(*filterTrunk),
		spamalot.FilterBranch(*filterBranch),
		spamalot.FilterMilestone(*filterMilestone),
		spamalot.WithPoW(pow),
		spamalot.WithSecurityLevel(spamalot.SecurityLevel(*securityLvl)),
		spamalot.WithTimeout(time.Duration(*timeout)*time.Second),
	)

	if err != nil {
		log.Println(err)
		return
	}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		s.Stop()
	}()

	s.Start()
}
