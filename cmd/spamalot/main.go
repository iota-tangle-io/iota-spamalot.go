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
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/coreos/bbolt"
	"github.com/cwarner818/giota"
	spamalot "github.com/iota-tangle-io/iota-spamalot.go"
	"github.com/kr/pretty"
	flag "github.com/spf13/pflag"
)

var (
	useNodes       *[]string = flag.StringSlice("node", []string{"http://localhost:14625"}, "remote IRI node")
	remoteNodeList *string   = flag.String("nodelist", "", "URL to fetch a list of IRI nodes from")
	mwm            *int64    = flag.Int64("mwm", 14, "minimum weight magnitude")
	depth          *int64    = flag.Int64("depth", giota.Depth, "the milestone depth used by the MCMC")
	timeout        *int64    = flag.Int64("timeout", 0, "how long to let the spammer run in seconds (if not specified infinite)")
	cooldown       *int64    = flag.Int64("cooldown", 0, "cooldown between spam TXs")
	securityLvl    *int64    = flag.Int64("security-lvl", 2, "the security lvl used for generating addresses")

	destAddress *string = flag.String("dest",
		"SPPRLTTIVYUONPOPQSWGCPMZWDOMQGWFUEPKUQIVUKROCHRNCR9MXNGNQSAGLKUDX9MZQWCPFJQS9DWAY", "address to send to")

	tag *string = flag.String("tag", "999SPAMALOT", "transaction tag")
	msg *string = flag.String("msg", "GOSPAMMER9VERSION9ONE9THREE", "transaction message")

	remotePow *bool = flag.Bool("pow", false,
		"if set, do PoW calculation on remote node via API")

	localPoW *bool = flag.Bool("local-pow", true,
		"if set, do PoW calculation locally")

	filterNonRemotePoWNodes *bool = flag.Bool("only-with-pow", false,
		"if set, filter out nodes from --nodelist which don't support remote PoW")

	verboseLogging *bool = flag.Bool("verbose", false,
		"if set, log various information to console about the spammer's state")

	strategy *string = flag.String("strategy", "non zero promote", "strategy to use for spamming")
)

type Node struct {
	Hostname                  string
	Port                      int
	LatestMilestoneIndex      int
	LatestSolidSubtangleIndex int
	Load                      int
	Ping                      int
	FreeMemory                int
	MaxMemory                 int
	Processors                int
	Version                   string
	Neighbors                 int
}

func checkNode(url string) (*spamalot.Node, error) {
	canAttach, err := canAttach(url)
	if err != nil {
		return nil, err
	}

	return &spamalot.Node{
		URL:            url,
		AttachToTangle: canAttach,
	}, nil
}

func main() {
	flag.Parse()

	var pow giota.PowFunc
	var powName string
	if !*remotePow {
		powName, pow = giota.GetBestPoW()
		log.Println("Using PoW:", powName)
	}

	nodes := make(map[string]bool)

	// check whether a remote node list was provided
	if remoteNodeList != nil && *remoteNodeList != "" {
		var hosts []Node
		err := getJson(*remoteNodeList, &hosts)
		if err != nil {
			log.Println("Unable to fetch host list:", err)
			return
		}
		//log.Println(len(hosts), "hosts loaded from", *remoteNodeList)
		for _, host := range hosts {
			url := "http://" + host.Hostname + ":" + strconv.Itoa(host.Port)
			nodes[url] = false
		}
	}

	// add manually specified nodes to the map
	if len(*useNodes) > 0 {
		for _, host := range *useNodes {
			nodes[host] = false
		}
	}

	// create in-memory nodes and check whether each node supports remote PoW
	log.Println("Checking", len(nodes), "nodes for AttachToTangle support")
	nodeChan := make(chan spamalot.Node, len(nodes))
	var wg sync.WaitGroup
	for url, _ := range nodes {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			n, err := checkNode(url)
			if *verboseLogging {
				pretty.Print(n, "\n")
			}
			if err != nil {
				if *verboseLogging {
					log.Println("Error checking node:", n, err)
				}
				return
			}
			nodeChan <- *n
		}(url)
	}
	wg.Wait()
	var nodelist []spamalot.Node
	length := len(nodeChan)
	for i := 0; i < length; i++ {
		n := <-nodeChan
		if *filterNonRemotePoWNodes && !n.AttachToTangle {
			continue
		}
		nodelist = append(nodelist, n)
		//nodes[n.URL] = n.AttachToTangle
	}

	log.Println(len(nodelist), "nodes responded")
	log.Println(counter, "nodes support AttachToTangle")
	if *filterNonRemotePoWNodes {
		log.Println("will only use nodes which support remote PoW")
	}
	db, err := bolt.Open("spamalot.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	database := spamalot.NewDatabase(db)
	s, err := spamalot.New(
		spamalot.WithNodes(nodelist),
		spamalot.WithMWM(*mwm),
		spamalot.WithDepth(*depth),
		spamalot.ToAddress(*destAddress),
		spamalot.WithTag(*tag),
		spamalot.WithMessage(*msg),
		spamalot.WithPoW(pow),
		spamalot.WithSecurityLevel(spamalot.SecurityLevel(*securityLvl)),
		spamalot.WithTimeout(time.Duration(*timeout)*time.Second),
		spamalot.WithCooldown(time.Duration(*cooldown)*time.Second),
		spamalot.WithVerboseLogging(*verboseLogging),
		spamalot.WithStrategy(*strategy),
		spamalot.WithLocalPoW(*localPoW),
		spamalot.WithDatabase(database),
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

// Send a garbage attachToTangle to the node and check the error to see if it
// supports it
var counter int

func canAttach(host string) (bool, error) {
	var errorResponse struct {
		Error string
	}

	request := []byte(`{"command": "attachToTangle", "trunkTransaction": "JVMTDGDPDFYHMZPMWEKKANBQSLSDTIIHAYQUMZOKHXXXGJHJDQPOMDOMNRDKYCZRUFZROZDADTHZC9999", "branchTransaction": "P9KFSJVGSPLXAEBJSHWFZLGP9GGJTIO9YITDEHATDTGAFLPLBZ9FOFWWTKMAZXZHFGQHUOXLXUALY9999", "minWeightMagnitude": 18, "trytes": ["TRYTVALUEHERE"]}`)
	req, err := http.NewRequest("POST", host, bytes.NewBuffer(request))
	req.Header.Set("X-IOTA-API-Version", "1")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	resp, err := client.Do(req)

	if err != nil {
		if *verboseLogging {
			log.Println("Error checking if host", host, "supports attachToTangle:", err)
		}
		return false, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	if err != nil {
		log.Println("Error unmarshalling json:", err)
		return false, err
	}

	if errorResponse.Error == "Invalid trytes input" {
		counter++
		return true, nil
	} else if errorResponse.Error != "COMMAND attachToTangle is not available on this node" {
		log.Println(host, errorResponse.Error)
	}
	return false, nil
}

// fetch JSON from the URL and unmarshal it in to the target
func getJson(url string, target interface{}) error {
	r, err := http.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	return json.NewDecoder(r.Body).Decode(target)
}
