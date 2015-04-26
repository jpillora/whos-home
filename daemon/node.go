package daemon

import "net"

type Node struct {
	MAC net.HardwareAddr
	IP  net.IP
}

type NodeQueue chan *Node

type NodeSet map[string]string
