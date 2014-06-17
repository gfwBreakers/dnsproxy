package main

import (
	"crypto/tls"
	"fmt"
	"github.com/miekg/dns"
	"net"
	"net/rpc"
	"time"
)

type DnsArgs struct {
	Msg     []byte
	Network string
}

type DnsProxyServer struct{}

func (s *DnsProxyServer) DnsRequest(args *DnsArgs, reply *[]byte) error {
	c := &dns.Client{Net: args.Network, ReadTimeout: time.Minute}
	req := new(dns.Msg)
	if err := req.Unpack(args.Msg); err != nil {
		conf.err.Print("request unpack error: ", err)
		return err
	}

	resp, _, err := c.Exchange(req, conf.ForwardDns)
	if err != nil {
		conf.err.Print("dns forward error: ", err)
		return err
	}

	*reply, err = resp.Pack()

	return err
}

func Server() {
	s := new(DnsProxyServer)
	listen := fmt.Sprintf("0.0.0.0:%s", conf.Port)
	l, err := tls.Listen("tcp", listen, &conf.tlsConfig)
	if err != nil {
		conf.err.Print("server listen failed: ", err)
		return
	}

	connChan := make(chan net.Conn)
	quitChan := make(chan struct{})

	defer func() {
		l.Close()
		close(connChan)
		close(quitChan)
	}()

	go func(l net.Listener) {
		for {
			c, err := l.Accept()
			if err != nil {
				conf.err.Print("accept error: ", err)
				quitChan <- struct{}{}
				return
			}
			connChan <- c
		}
	}(l)

	for {
		select {
		case c := <-connChan:
			go func(c net.Conn) {
				defer c.Close()

				rpcServer := rpc.NewServer()
				rpcServer.Register(s)
				rpcServer.ServeConn(c)
			}(c)
		case <-quitChan:
			return
		}
	}
}
