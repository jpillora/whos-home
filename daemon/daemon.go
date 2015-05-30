package daemon

import (
	"bytes"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
)

type Config struct {
	Interfaces []string      `type:"arg" min:"1"`
	Interval   time.Duration `help:"the interval between scans"`
	Endpoint   string        `help:"an HTTP POST endpoint (defaults to send to stdout)"`
	SingleCore bool          `help:"only use a single core (defaults to multicore)"`
}

func Run(c Config) {

	//logs to stderr
	log.SetOutput(os.Stderr)

	cpu := runtime.NumCPU()
	runtime.GOMAXPROCS(cpu)
	log.Printf("whos-home initializing (#%d cores)", cpu)

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

	//in a goroutine, extract all nodes from queue
	go monitor(c.Endpoint, queue)

	//scan all provided interfaces, append all to queue
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
	//collect unique nodes (thread-safe)
	l := sync.Mutex{}
	nodes := NodeSet{}
	//send out all nodes every 5 seconds
	go func() {
		for {
			l.Lock()
			send(endpoint, nodes)
			for k, _ := range nodes {
				delete(nodes, k)
			}
			l.Unlock()
			time.Sleep(5 * time.Second)
		}
		time.Sleep(time.Second)
	}()
	//fill nodes map forever
	for node := range queue {
		l.Lock()
		nodes[node.MAC.String()] = node.IP.String()
		l.Unlock()
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
