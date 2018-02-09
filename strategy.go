package spamalot

import "github.com/CWarner818/giota"

// Not too sure how I want to define this interface, but I know I want to have it
type Strategy interface {
	GetTips() (*Tips, error)
}

type StrategyFunc func() (*Tips, error)

// Tips represents a pair of transactions to confirm
type Tips struct {
	Trunk, Branch         giota.Transaction
	TrunkHash, BranchHash giota.Trytes
}

func (w *Worker) GetTips() (*Tips, error) {

	return nil, nil
}
