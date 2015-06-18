package daemon

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
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
	CacheDNS   bool          `help:"cache dns responses"`
}

var DNScache = map[string]string(nil)

func Run(c Config) {

	//logs to stderr
	log.SetOutput(os.Stderr)

	if c.CacheDNS {
		DNScache = map[string]string{}
	}

	//get ip
	endpoint, err := url.Parse(c.Endpoint)
	if c.Endpoint == "" {
		endpoint = nil
	} else if err != nil {
		log.Fatal("Invalid endpoint: %s", err)
	}

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
	go monitor(endpoint, queue)

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

func monitor(endpoint *url.URL, queue NodeQueue) {
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

func send(endpoint *url.URL, nodes NodeSet) {

	if len(nodes) == 0 {
		return
	}

	b, err := json.MarshalIndent(nodes, "", "  ")

	//no endpoint, send to stdout
	if endpoint == nil {
		os.Stdout.Write(b)
		os.Stdout.WriteString("\n")
		return
	}

	req, _ := http.NewRequest("POST", endpoint.String(), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")

	//cache dns?
	var client *http.Client
	if DNScache == nil {
		client = http.DefaultClient
	} else {
		client = dnsCachingClient
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("send failed: %s", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		log.Printf("send error: %d: %s", resp.StatusCode, b)
		return
	}
}

var dnsCachingClient = &http.Client{
	Transport: &http.Transport{
		DialTLS: func(network, addr string) (net.Conn, error) {
			if network != "tcp" {
				return nil, errors.New("unsupported network")
			}
			h, p, _ := net.SplitHostPort(addr)
			ip, ok := DNScache[h]
			if !ok {
				ips, err := net.LookupIP(h)
				if err != nil {
					return nil, fmt.Errorf("DNS lookup failed: %s", err)
				}
				ip := ips[0].To4().String()
				log.Printf("DNS lookup: %s -> %s", h, ip)
				DNScache[h] = ip
			}
			return tls.Dial(network, ip+":"+p, &tls.Config{
				ServerName: h,
			})
		},
	},
}
