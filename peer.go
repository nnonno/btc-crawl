package main

import (
	"fmt"
	"net"
	"time"
	
	"github.com/btcsuite/btcd/wire"
)

type Peer struct {
	client          *Client
	conn            net.Conn
	nonce           uint64 // Nonce we're sending to the peer
	pver            uint32 // Negotiated ProtocolVersion
	Address         string
	UserAgent       string
	ProtocolVersion int32
	ConnectTimeout  time.Duration // For the connect phase (can be overridden)
}

func NewPeer(client *Client, address string) *Peer {
	p := Peer{
		client:         client,
		pver:           client.pver,
		Address:        address,
		ConnectTimeout: time.Duration(20 * time.Second),
	}
	return &p
}

func (p *Peer) Connect() error {
	if p.conn != nil {
		return fmt.Errorf("Peer already connected, can't connect again.")
	}
	conn, err := net.DialTimeout("tcp", p.Address, p.ConnectTimeout)
	if err != nil {
		return err
	}

	p.conn = conn
	return nil
}

func (p *Peer) Disconnect() {
	p.conn.Close()
	logger.Debugf("[%s] Closed.", p.Address)
}

func (p *Peer) Handshake() error {
	if p.conn == nil {
		return fmt.Errorf("Peer is not connected, can't handshake.")
	}

	logger.Debugf("[%s] Starting handshake.", p.Address)

	nonce, err := wire.RandomUint64()
	if err != nil {
		return err
	}
	p.nonce = nonce
	
// 	lhost, lstrPort, _ := net.SplitHostPort(p.conn.localaddr())
	lna, err := wire.NewNetAddress(&net.TCPAddr{IP: p.conn.localaddr(), Port: 8333}, 0)
	
// 	rhost, rstrPort, _ := net.SplitHostPort(p.conn.raddr)
	rna, err := wire.NewNetAddress(&net.TCPAddr{IP: p.connRemoteAddr(), Port: 8333}, 0)
	
	msgVersion := wire.NewMsgVersion(lna, rna, p.nonce, 0)
	msgVersion.UserAgent = p.client.userAgent
	msgVersion.DisableRelayTx = true
	if err := p.WriteMessage(msgVersion); err != nil {
		return err
	}

	// Read the response version.
	msg, _, err := p.ReadMessage()
	if err != nil {
		return err
	}
	vmsg, ok := msg.(*wire.MsgVersion)
	if !ok {
		return fmt.Errorf("Did not receive version message: %T", vmsg)
	}

	p.ProtocolVersion = vmsg.ProtocolVersion
	p.UserAgent = vmsg.UserAgent

	// Negotiate protocol version.
	if uint32(vmsg.ProtocolVersion) < p.pver {
		p.pver = uint32(vmsg.ProtocolVersion)
	}
	logger.Debugf("[%s] -> Version: %s", p.Address, vmsg.UserAgent)

	// Normally we'd check if vmsg.Nonce == p.nonce but the crawler does not
	// accept external connections so we skip it.

	// Send verack.
	if err := p.WriteMessage(wire.NewMsgVerAck()); err != nil {
		return err
	}

	return nil
}

func (p *Peer) WriteMessage(msg wire.Message) error {
	return wire.WriteMessage(p.conn, msg, p.pver, p.client.btcnet)
}

func (p *Peer) ReadMessage() (wire.Message, []byte, error) {
	return wire.ReadMessage(p.conn, p.pver, p.client.btcnet)
}
