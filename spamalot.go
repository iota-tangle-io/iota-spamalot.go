package spamalot

import (
	"errors"

	"github.com/cwarner818/giota"
)

type Spammer struct {
	Node         string
	MWM          int64
	Depth        int64
	DestAddress  string
	Tag          string
	Message      string
	FilterTrunk  bool
	FilterBranch bool
	RemotePow    bool

	badBranch, badTrunk, badBoth int
}

func New() *Spammer {
	return &Spammer{}
}

func Close() error {
	return nil
}

func (s *Spammer) SendTrytes(api *giota.API, depth int64, trytes []giota.Transaction, mwm int64, pow giota.PowFunc) error {
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

	paddedTag := padTag(s.Tag)
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
		if s.FilterTrunk || s.FilterBranch {
			return errors.New("Trunk and branch txn tag is ours")
		}
	} else if trunkIsBad {
		s.badTrunk++
		if s.FilterTrunk {
			return errors.New("Trunk txn tag is ours")
		}
	} else if branchIsBad {
		s.badBranch++
		if s.FilterBranch {
			return errors.New("Branch txn tag is ours")
		}
	}

	switch {
	case s.RemotePow || pow == nil:
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
