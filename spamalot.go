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
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/CWarner818/giota"
)

type Node struct {
	URL            string
	AttachToTangle bool
}

type SecurityLevel byte

const (
	SECURITY_LVL_LOW    SecurityLevel = 1 // 81-trits (low)
	SECURITY_LVL_MEDIUM SecurityLevel = 2 // 162-trits (medium)
	SECURITY_LVL_HIGH   SecurityLevel = 3 // 243-trits (high)
)

type Spammer struct {
	sync.RWMutex

	nodes       []Node
	mwm         int64
	depth       int64
	securityLvl SecurityLevel
	destAddress string
	tag         string
	message     string

	filterTrunk     bool
	filterBranch    bool
	filterMilestone bool

	remotePoW bool

	localPoW bool

	pow giota.PowFunc
	wg  sync.WaitGroup

	tipsChan chan Tips
	powMu    sync.Mutex

	txsChan    chan Transaction
	stopSignal chan struct{}
	timeout    time.Duration
	cooldown   time.Duration

	verboseLogging bool
	running        bool

	strategy    string
	metrics     *metricsrouter
	metricRelay chan<- Metric
}

func (s *Spammer) NewWorker(node Node) *Worker {
	return &Worker{
		node:       node,
		api:        giota.NewAPI(node.URL, nil),
		spammer:    s,
		stopSignal: s.stopSignal,
	}
}

type Option func(*Spammer) error

func New(options ...Option) (*Spammer, error) {
	s := &Spammer{}
	for _, option := range options {
		err := option(s)

		if err != nil {
			return nil, err
		}
	}
	return s, nil
}
func (s *Spammer) UpdateSettings(options ...Option) error {
	s.Lock()
	defer s.Unlock()
	for _, option := range options {
		err := option(s)

		return err
	}
	return nil
}

func WithStrategy(strategy string) Option {
	return func(s *Spammer) error {
		s.strategy = strategy
		return nil
	}
}
func WithNodes(nodes []Node) Option {
	return func(s *Spammer) error {
		s.nodes = append(s.nodes, nodes...)
		return nil
	}
}
func WithNode(node string, attachToTangle bool) Option {
	return func(s *Spammer) error {
		s.nodes = append(s.nodes, Node{URL: node, AttachToTangle: attachToTangle})
		return nil
	}
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
func WithPoW(pow giota.PowFunc) Option {
	return func(s *Spammer) error {
		s.pow = pow
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
func FilterMilestone(filter bool) Option {
	return func(s *Spammer) error {
		s.filterMilestone = filter
		return nil
	}
}

func WithVerboseLogging(verboseLogging bool) Option {
	return func(s *Spammer) error {
		s.verboseLogging = verboseLogging
		return nil
	}
}

func WithLocalPoW(localPoW bool) Option {
	return func(s *Spammer) error {
		s.localPoW = localPoW
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

func WithSecurityLevel(securityLvl SecurityLevel) Option {
	return func(s *Spammer) error {
		s.securityLvl = securityLvl
		return nil
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(s *Spammer) error {
		s.timeout = timeout
		return nil
	}
}

func WithCooldown(cooldown time.Duration) Option {
	return func(s *Spammer) error {
		s.cooldown = cooldown
		return nil
	}
}

func WithMetricsRelay(relay chan<- Metric) Option {
	return func(s *Spammer) error {
		s.metricRelay = relay
		return nil
	}
}

func (s *Spammer) Close() error {
	return s.Stop()
}

func (s *Spammer) logIfVerbose(str ...interface{}) {
	if s.verboseLogging {
		log.Println(str...)
	}
}

func (s *Spammer) Start() {
	log.Println("IOTΛ Spamalot starting")
	seed := giota.NewSeed()

	recipientT, err := giota.ToAddress(s.destAddress)
	if err != nil {
		panic(err)
	}

	ttag, err := giota.ToTrytes(s.tag)
	if err != nil {
		panic(err)
	}

	tmsg, err := giota.ToTrytes(s.message)
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

	log.Println("Using IRI nodes:", s.nodes)

	s.txsChan = make(chan Transaction, 50)
	s.tipsChan = make(chan Tips, 50)
	s.stopSignal = make(chan struct{})
	s.metrics = newMetricsRouter()

	if s.metricRelay != nil {
		s.metrics.addRelay(s.metricRelay)
	}

	go s.metrics.collect()
	defer s.metrics.stop()

	if s.timeout != 0 {
		go func() {
			<-time.After(s.timeout)
			s.Stop()
		}()
	}

	for _, node := range s.nodes {
		w := Worker{
			node:       node,
			api:        giota.NewAPI(node.URL, nil),
			spammer:    s,
			stopSignal: s.stopSignal,
		}
		switch strings.ToLower(s.strategy) {
		case "non zero promote":
			go w.getNonZeroTips(s.tipsChan, &s.wg)
		case "":
			go w.getTxnsToApprove(s.tipsChan, &s.wg)
		default:
			log.Println("Unknown strategy `" + s.strategy + "'")
			return
		}
		s.wg.Add(2)
		go w.spam(s.txsChan, &s.wg)
	}

	s.running = true
	defer func() {
		log.Println("Waiting for workers to terminate...")
		s.wg.Wait()
		s.running = false
	}()

	// iterate randomly over available nodes and create
	// shallow txs to send to workers for processing
	for {
		select {
		case <-s.stopSignal:
			return
		default:
			node := s.nodes[rand.Intn(len(s.nodes))]
			api := giota.NewAPI(node.URL, nil)
			bdl, err = giota.PrepareTransfers(api, seed, trs, nil, "", int(s.securityLvl))
			if err != nil {
				s.metrics.addMetric(INC_FAILED_TX, nil)
				s.logIfVerbose("Error preparing transfer:", err)
				continue
			}

			txns, err := s.buildTransactions(bdl, s.pow)
			if err != nil {
				s.metrics.addMetric(INC_FAILED_TX, nil)
				s.logIfVerbose("Error building txn", node.URL, err)
				continue
			}

			// if the built transaction is nil here, the buildTransactions() function
			// was instructed to stop by a stop signal
			if txns == nil {
				return
			}

			// send shallow tx to worker or exit if signaled
			select {
			case <-s.stopSignal:
				return
			case s.txsChan <- *txns:
			}
		}
	}
}

func (s *Spammer) Stop() error {
	// nil the tip and txs channel so that send/receive
	// on those channels becomes blocking
	s.txsChan = nil
	s.tipsChan = nil

	// once for tip and once for spam goroutine per node + main loop
	for i := 0; i < len(s.nodes)*2+1; i++ {
		s.stopSignal <- struct{}{}
	}

	// close the stop signal channel so that every select auto unwinds
	close(s.stopSignal)
	return nil
}

func (s *Spammer) IsRunning() bool {
	s.RLock()
	defer s.RUnlock()
	return s.running
}

type Transaction struct {
	Trunk, Branch giota.Trytes
	Transactions  []giota.Transaction
}

const milestoneAddr = "KPWCHICGJZXKE9GSUDXZYUAPLHAKAHYHDXNPHENTERYMMBQOPSQIDENXKLKCEYCPVTZQLEEJVYJZV9BWU"

func (s *Spammer) buildTransactions(trytes []giota.Transaction, pow giota.PowFunc) (*Transaction, error) {

	/*
		tra, err := api.GetTransactionsToApprove(s.depth)
		if err != nil {
			log.Println("GetTransactionsToApprove error", err)
			return nil, err
		}

		txns, err := api.GetTrytes([]giota.Trytes{
			tra.TrunkTransaction,
			tra.BranchTransaction,
		})

		if err != nil {
			return nil, err
		}

	*/

	select {
	// if the stop signal is received in here, the main loop
	// will also break because a nil tx indicates stopping
	case <-s.stopSignal:
		return nil, nil

	case tips := <-s.tipsChan:
		paddedTag := padTag(s.tag)
		tTag := string(tips.Trunk.Tag)
		bTag := string(tips.Branch.Tag)

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

		if strings.Contains(string(tips.Trunk.Address), milestoneAddr) {
			s.metrics.addMetric(INC_MILESTONE_TRUNK, nil)
			if s.filterMilestone {
				return nil, errors.New("Trunk txn is a milestone")
			}

		} else if strings.Contains(string(tips.Branch.Address), milestoneAddr) {
			s.metrics.addMetric(INC_MILESTONE_BRANCH, nil)
			if s.filterMilestone {
				return nil, errors.New("Branch txn is a milestone")
			}
		}

		if bothAreBad {
			s.metrics.addMetric(INC_BAD_TRUNK_AND_BRANCH, nil)
			if s.filterTrunk || s.filterBranch {
				return nil, errors.New("Trunk and branch txn tag is ours")
			}
		} else if trunkIsBad {
			s.metrics.addMetric(INC_BAD_TRUNK, nil)
			if s.filterTrunk {
				return nil, errors.New("Trunk txn tag is ours")
			}
		} else if branchIsBad {
			s.metrics.addMetric(INC_BAD_BRANCH, nil)
			if s.filterBranch {
				return nil, errors.New("Branch txn tag is ours")
			}
		}

		return &Transaction{
			Trunk:        tips.TrunkHash,
			Branch:       tips.BranchHash,
			Transactions: trytes,
		}, nil
	}

}
