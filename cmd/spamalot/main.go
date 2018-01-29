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

	"github.com/cwarner818/giota"
	spamalot "github.com/iota-tangle-io/iota-spamalot.go"
	flag "github.com/spf13/pflag"
)

var (
	mwm   *int64 = flag.Int64("mwm", 14, "minimum weight magnitude")
	depth *int64 = flag.Int64("depth", giota.Depth, "whatever depth is")

	destAddress *string = flag.String("dest",
		"SPPRLTTIVYUONPOPQSWGCPMZWDOMQGWFUEPKUQIVUKROCHRNCR9MXNGNQSAGLKUDX9MZQWCPFJQS9DWAY", "address to send to")

	tag *string = flag.String("tag", "999SPAMALOT", "transaction tag")
	msg *string = flag.String("msg", "GOSPAMMER9VERSION9ONE9TWO", "transaction message")
	//nodes *[]string = flag.StringSlice("node", []string{"http://localhost:14265"}, "remote node to connect to")

	filterTrunk *bool = flag.Bool("trunk", false,
		"do not send a transaction with our own transaction as a trunk")

	filterBranch *bool = flag.Bool("branch", false,
		"do not send a transaction with our own transaction as a branch")

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
		spamalot.WithNode("http://localhost:14625", false),
		spamalot.WithMWM(*mwm),
		spamalot.WithDepth(*depth),
		spamalot.ToAddress(*destAddress),
		spamalot.WithTag(*tag),
		spamalot.WithMessage(*msg),
		spamalot.FilterTrunk(*filterTrunk),
		spamalot.FilterBranch(*filterBranch),
		spamalot.WithPoW(pow),
	)

	if err != nil {
		log.Println(err)
		return
	}

	s.Start()
}
