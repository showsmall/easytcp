package server

import (
	"fmt"
	"github.com/DarthPestilane/easytcp/logger"
	"github.com/DarthPestilane/easytcp/packet"
	"github.com/DarthPestilane/easytcp/router"
	"github.com/DarthPestilane/easytcp/session"
	"github.com/sirupsen/logrus"
	"net"
)

type UdpServer struct {
	conn          *net.UDPConn
	rwBufferSize  int
	maxBufferSize int
	log           *logrus.Entry
	msgPacker     packet.Packer
	msgCodec      packet.Codec
	accepting     chan struct{}
	stopped       chan struct{}
}

type UdpOption struct {
	MaxBufferSize int
	RWBufferSize  int
	MsgPacker     packet.Packer
	MsgCodec      packet.Codec
}

func NewUdp(opt UdpOption) *UdpServer {
	if opt.MaxBufferSize <= 0 {
		opt.MaxBufferSize = 1024
	}
	if opt.MsgPacker == nil {
		opt.MsgPacker = &packet.DefaultPacker{}
	}
	if opt.MsgCodec == nil {
		opt.MsgCodec = &packet.StringCodec{}
	}
	return &UdpServer{
		log:           logger.Default.WithField("scope", "server.UdpServer"),
		rwBufferSize:  opt.RWBufferSize,
		msgPacker:     opt.MsgPacker,
		msgCodec:      opt.MsgCodec,
		maxBufferSize: opt.MaxBufferSize,
		accepting:     make(chan struct{}),
		stopped:       make(chan struct{}),
	}
}

func (t *UdpServer) Serve(addr string) error {
	address, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", address)
	if err != nil {
		return err
	}
	if t.rwBufferSize > 0 {
		if err := conn.SetReadBuffer(t.rwBufferSize); err != nil {
			return fmt.Errorf("conn set read buffer err: %s", err)
		}
		if err := conn.SetWriteBuffer(t.rwBufferSize); err != nil {
			return fmt.Errorf("conn set write buffer err: %s", err)
		}
	}
	t.conn = conn
	return t.acceptLoop()
}

func (t *UdpServer) acceptLoop() error {
	close(t.accepting)
	buff := make([]byte, t.maxBufferSize)
	for {
		n, remoteAddr, err := t.conn.ReadFromUDP(buff)
		if err != nil {
			return fmt.Errorf("read conn err: %s", err)
		}
		go t.handleIncomingMsg(buff[:n], remoteAddr)
	}
}

func (t *UdpServer) handleIncomingMsg(msg []byte, addr *net.UDPAddr) {
	sess := session.NewUdp(t.conn, addr, t.msgPacker, t.msgCodec)
	defer func() { t.log.WithField("sid", sess.ID()).Tracef("session closed") }()

	go router.Instance().Loop(sess)
	if err := sess.ReadIncomingMsg(msg); err != nil {
		return
	}
	sess.Write(t.stopped)
	sess.Close()
}

func (t *UdpServer) Stop() error {
	close(t.stopped)
	return t.conn.Close()
}