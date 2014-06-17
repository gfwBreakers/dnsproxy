package main

import (
	"crypto/tls"
	"fmt"
	"github.com/miekg/dns"
	"net"
	"net/rpc"
	"sync"
)

type ClientDns struct {
	c   net.Conn
	rpc *rpc.Client
}

func (c *ClientDns) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	network := "udp"
	if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		network = "tcp"
	}

	m := new(dns.Msg)
	var err error
	defer func() {
		if err != nil {
			m.SetReply(req)
			m.SetRcode(req, dns.RcodeServerFailure)
			w.WriteMsg(m)
		}
	}()

	var reqBuf, respBuf []byte
	reqBuf, err = req.Pack()
	if err != nil {
		conf.err.Print("pack request error:", err)
		return
	}

	args := &DnsArgs{
		Msg:     reqBuf,
		Network: network,
	}
	err = c.rpc.Call("DnsProxyServer.DnsRequest", args, &respBuf)
	if err != nil {
		conf.err.Print("call rpc error:", err)
		return
	}
	err = m.Unpack(respBuf)
	if err != nil {
		conf.err.Print("unpack response error:", err)
		return
	}

	w.WriteMsg(m)
}

func Client() {
	rpcAddr := fmt.Sprintf("%s:%s", conf.Server, conf.Port)
	c, err := tls.Dial("tcp", rpcAddr, &conf.tlsConfig)
	if err != nil {
		conf.err.Print("dial", rpcAddr, "error:", err)
		return
	}

	client := rpc.NewClient(c)

	mux := dns.NewServeMux()
	mux.Handle(".", &ClientDns{
		c:   c,
		rpc: client,
	})

	var group sync.WaitGroup
	group.Add(2)

	for _, net := range []string{"tcp", "udp"} {
		go func(net string) {
			defer group.Done()

			server := &dns.Server{
				Addr:    conf.ForwardDns,
				Net:     net,
				Handler: mux,
			}
			if err := server.ListenAndServe(); err != nil {
				conf.err.Print("dns server launch", net, "error:", err)
				return
			}
		}(net)
	}

	group.Wait()
}
