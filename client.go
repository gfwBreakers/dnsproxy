package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/miekg/dns"
	"net"
	"net/rpc"
	"strings"
	"sync"
	"time"
)

type ClientDns struct {
	c          net.Conn
	rpc        *rpc.Client
	dnsClients map[string]*dns.Client
}

func NewClientDns() (*ClientDns, error) {
	rpcAddr := fmt.Sprintf("%s:%s", conf.Server, conf.Port)
	c, err := tls.Dial("tcp", rpcAddr, &conf.tlsConfig)
	if err != nil {
		conf.err.Print("dial error: ", err)
		return nil, err
	}

	client := rpc.NewClient(c)
	return &ClientDns{
		c:   c,
		rpc: client,
		dnsClients: map[string]*dns.Client{
			"tcp": &dns.Client{Net: "tcp", ReadTimeout: time.Minute},
			"udp": &dns.Client{Net: "udp", ReadTimeout: time.Minute},
		},
	}, nil
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

	localForward := true
	domainBuf := bytes.NewBuffer(nil)
	for _, question := range req.Question {
		name := strings.Trim(question.Name, ".")
		domainBuf.WriteString(name)
		domainBuf.WriteString("|")
		if !conf.domainFilter.MatchString(name) {
			localForward = false
			break
		}
	}
	if localForward {
		conf.info.Printf("|local|%s", domainBuf.String())
		client, ok := c.dnsClients[network]
		if !ok {
			conf.err.Print("unknown network: ", network)
			err = fmt.Errorf("unknown network: %s", network)
			return
		}
		var resp *dns.Msg
		resp, _, err = client.Exchange(req, conf.ForwardDns)
		if err != nil {
			conf.err.Print("dns forward error: ", err)
			return
		}
		w.WriteMsg(resp)
		return
	}

	conf.info.Printf("|forward|%s", domainBuf.String())
	var reqBuf, respBuf []byte
	reqBuf, err = req.Pack()
	if err != nil {
		conf.err.Print("pack request error: ", err)
		return
	}

	args := &DnsArgs{
		Msg:     reqBuf,
		Network: network,
	}
	err = c.rpc.Call("DnsProxyServer.DnsRequest", args, &respBuf)
	if err != nil {
		conf.err.Print("call rpc error: ", err)
		return
	}
	err = m.Unpack(respBuf)
	if err != nil {
		conf.err.Print("unpack response error: ", err)
		return
	}

	w.WriteMsg(m)
}

func Client() {
	client, err := NewClientDns()
	if err != nil {
		return
	}

	mux := dns.NewServeMux()
	mux.Handle(".", client)

	var group sync.WaitGroup
	group.Add(2)

	for _, net := range []string{"tcp", "udp"} {
		go func(net string) {
			defer group.Done()

			server := &dns.Server{
				Addr:    conf.LocalDns,
				Net:     net,
				Handler: mux,
			}
			if err := server.ListenAndServe(); err != nil {
				conf.err.Print("dns server launch ", net, " error: ", err)
				return
			}
		}(net)
	}

	group.Wait()
}
