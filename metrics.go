package spamalot

import (
	"time"
	"log"
	"github.com/CWarner818/giota"
)

type MetricType byte

const (
	INC_MILESTONE_BRANCH     MetricType = 0
	INC_MILESTONE_TRUNK      MetricType = 1
	INC_BAD_TRUNK            MetricType = 2
	INC_BAD_BRANCH           MetricType = 3
	INC_BAD_TRUNK_AND_BRANCH MetricType = 4
	INC_FAILED_TX            MetricType = 5
	INC_SUCCESSFUL_TX        MetricType = 6
)

type metric struct {
	kind MetricType
	data interface{}
}

type txandnode struct {
	tx   Transaction
	node Node
}

func newMetricsRouter() *metricsrouter {
	return &metricsrouter{
		metrics:    make(chan metric),
		stopSignal: make(chan struct{}),
	}
}

type metricsrouter struct {
	metrics    chan metric
	stopSignal chan struct{}

	startTime time.Time

	txsSucceeded, txsFailed, badBranch, badTrunk, badTrunkAndBranch int
	milestoneTrunk, milestoneBranch                                 int
}

func (mr *metricsrouter) stop() {
	mr.metrics = nil
	mr.stopSignal <- struct{}{}
}

func (mr *metricsrouter) addMetric(kind MetricType, data interface{}) {
	mr.metrics <- metric{kind, data}
}

func (mr *metricsrouter) collect() {
	mr.startTime = time.Now()
exit:
	for {
		select {
		case <-mr.stopSignal:
			break exit
		case metric := <-mr.metrics:
			switch metric.kind {
			case INC_MILESTONE_BRANCH:
				mr.milestoneBranch++
			case INC_MILESTONE_TRUNK:
				mr.milestoneTrunk++
			case INC_BAD_TRUNK:
				mr.badTrunk++
			case INC_BAD_BRANCH:
				mr.badBranch++
			case INC_BAD_TRUNK_AND_BRANCH:
				mr.badTrunkAndBranch++
			case INC_FAILED_TX:
				mr.txsFailed++
			case INC_SUCCESSFUL_TX:
				mr.txsSucceeded++
				mr.printMetrics(metric.data.(txandnode))
			}
		}
	}
}

func (mr *metricsrouter) printMetrics(txAndNode txandnode) {
	tx := txAndNode.tx
	node := txAndNode.node

	if len(tx.Transactions) > 1 {
		log.Println("Bundle sent to", node,
			"\nhttp://thetangle.org/bundle/"+giota.Bundle(tx.Transactions).Hash())
	} else {
		log.Println("Txn sent to", node,
			"\nhttp://thetangle.org/transaction/"+tx.Transactions[0].Hash())
	}

	// TPS = delta since startup / successful TXs
	dur := time.Since(mr.startTime)
	tps := float64(mr.txsSucceeded) / dur.Seconds()

	// success rate = successful TXs / successful TXs + failed TXs
	successRate := 100 * (float64(mr.txsSucceeded) / (float64(mr.txsSucceeded) + float64(mr.txsFailed)))
	log.Printf("%.2f TPS -- success rate %.0f%% ", tps, successRate)

	log.Printf("Duration: %s Count: %d Milestone Trunk: %d Milestone Branch: %d Bad Trunk: %d Bad Branch: %d Both: %d",
		dur.String(), mr.txsSucceeded, mr.milestoneTrunk,
		mr.milestoneBranch, mr.badTrunk, mr.badBranch, mr.badTrunkAndBranch)
}
