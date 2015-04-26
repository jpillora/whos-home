package daemon

import (
	"bytes"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

type Config struct {
	Interfaces []string      `min:"1"`
	Interval   time.Duration `help:"the interval between scans"`
	Endpoint   string        `help:"an HTTP POST endpoint (defaults to send to stdout)"`
}

func Run(c Config) {
	//get all in list
	ifaces := []*net.Interface{}
	for _, n := range c.Interfaces {
		iface, err := net.InterfaceByName(n)
		if err != nil {
			log.Printf("could not get interface %s: %s", n, err)
			return
		}
		ifaces = append(ifaces, iface)
	}

	//prepare queue
	queue := make(NodeQueue)
	go monitor(c.Endpoint, queue)

	//scan all
	var wg sync.WaitGroup
	for _, iface := range ifaces {
		wg.Add(1)
		// Start up a scan on each interface.
		func(iface *net.Interface) {
			defer wg.Done()
			if err := scan(iface, c.Interval, queue); err != nil {
				log.Printf("%s error: %s", iface.Name, err)
			}
		}(iface)
	}

	wg.Wait()
	return
}

func monitor(endpoint string, queue NodeQueue) {
	//collect unique nodes
	nodes := NodeSet{}
	//send out all nodes every 5 seconds
	go func() {
		for {
			send(endpoint, nodes)
			for k, _ := range nodes {
				delete(nodes, k)
			}
			time.Sleep(5 * time.Second)
		}
		time.Sleep(time.Second)
	}()
	//fill nodes map forever
	for node := range queue {
		nodes[node.MAC.String()] = node.IP.String()
	}
}

func send(endpoint string, nodes NodeSet) {

	if len(nodes) == 0 {
		return
	}

	b, err := json.MarshalIndent(nodes, "", "  ")

	//no endpoint, send to stdout
	if endpoint == "" {
		os.Stdout.Write(b)
		os.Stdout.WriteString("\n")
		return
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewBuffer(b))
	if err != nil {
		log.Printf("send failed: %s", err)
		return
	}

	if resp.StatusCode != 200 {
		log.Printf("send error: %d", resp.StatusCode)
		return
	}
}
