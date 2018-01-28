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

// spamalot description here. writing documentation is no fun.
package spamalot

import (
	"errors"
	"sync"

	"github.com/cwarner818/giota"
)

const ()

type Spammer struct {
	sync.RWMutex
	nodes       []string
	mwm         int64
	depth       int64
	destAddress string
	tag         string
	message     string

	filterTrunk  bool
	filterBranch bool

	remotePoW bool

	badBranch, badTrunk, badBoth int
}

type Option func(*Spammer) error

func New(nodes []string, options ...Option) (*Spammer, error) {
	if len(nodes) == 0 {
		return nil, errors.New("You must specify at least one node to connect to")
	}
	s := &Spammer{
		nodes: nodes,
	}
	for _, option := range options {
		err := option(s)

		return s, err
	}
	return s, nil
}
func (s *Spammer) UpdateConfig(options ...Option) error {
	s.Lock()
	defer s.Unlock()
	for _, option := range options {
		err := option(s)
		return err
	}
	return nil
}
func WithMessage(msg string) Option {
	return func(s *Spammer) error {
		// TODO: check msg for validity
		s.message = msg
		return nil
	}
}
func WithTag(tag string) Option {
	return func(s *Spammer) error {
		// TODO: check tag for validity
		s.tag = tag
		return nil
	}
}
func ToAddress(addr string) Option {
	return func(s *Spammer) error {
		// TODO: Check address for validity
		s.destAddress = addr
		return nil
	}
}
func WithRemotePoW(remotePoW bool) Option {
	return func(s *Spammer) error {
		s.remotePoW = remotePoW
		return nil
	}
}
func FilterTrunk(filter bool) Option {
	return func(s *Spammer) error {
		s.filterTrunk = filter
		return nil
	}
}
func FilterBranch(filter bool) Option {
	return func(s *Spammer) error {
		s.filterBranch = filter
		return nil
	}
}
func WithDepth(depth int64) Option {
	return func(s *Spammer) error {
		s.depth = depth
		return nil
	}
}

func WithMWM(mwm int64) Option {
	return func(s *Spammer) error {
		s.mwm = mwm
		return nil
	}
}

func (s *Spammer) Close() error {
	return nil
}

func (s *Spammer) SendTrytes(api *giota.API, depth int64, trytes []giota.Transaction, mwm int64, pow giota.PowFunc) error {
	if s == nil {
		return errors.New("cannot SendTrytes with nil Spammer")
	}
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

	paddedTag := padTag(s.tag)
	tTag := string(txns.Trytes[0].Tag)
	bTag := string(txns.Trytes[1].Tag)

	var branchIsBad, trunkIsBad, bothAreBad bool
	if bTag == paddedTag {
		branchIsBad = true
	}

	if tTag == paddedTag {
		trunkIsBad = true
	}

	if trunkIsBad && branchIsBad {
		bothAreBad = true
	}

	if bothAreBad {
		s.badBoth++
		if s.filterTrunk || s.filterBranch {
			return errors.New("Trunk and branch txn tag is ours")
		}
	} else if trunkIsBad {
		s.badTrunk++
		if s.filterTrunk {
			return errors.New("Trunk txn tag is ours")
		}
	} else if branchIsBad {
		s.badBranch++
		if s.filterBranch {
			return errors.New("Branch txn tag is ours")
		}
	}

	switch {
	case s.remotePoW || pow == nil:
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

func padTag(tag string) string {
	for {
		tag += "9"
		if len(tag) > 27 {
			return tag[0:27]
		}
	}
}
