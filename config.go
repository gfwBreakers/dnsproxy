package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"github.com/googollee/yconf"
	"io"
	"io/ioutil"
	"log"
	"log/syslog"
	"os"
)

type Config struct {
	Server     string `yaml:"server"`
	Port       string `yaml:"port"`
	ForwardDns string `yaml:"forward_dns"`
	LocalDns   string `yaml:"local_dns"`
	Mode       string `yaml:"mode"`
	CertPem    string `yaml:"cert_pem"`
	KeyPem     string `yaml:"key_pem"`
	ErrorLog   string `yaml:"error_log"`
	InfoLog    string `yaml:"info_log"`
	DebugLog   string `yaml:"debug_log"`

	tlsConfig tls.Config
	err       *log.Logger
	info      *log.Logger
	debug     *log.Logger
}

var conf Config

func Init() error {
	var confFile string
	flag.StringVar(&confFile, "conf", "./conf.yaml", "configure file name")
	flag.Parse()

	f, err := os.Open(confFile)
	if err != nil {
		fmt.Println("can't open conf file:", err)
		return err
	}
	if err := yconf.Unmarshal(f, &conf); err != nil {
		fmt.Println("parse confi file error:", err)
		return err
	}
	cert, err := tls.LoadX509KeyPair(conf.CertPem, conf.KeyPem)
	if err != nil {
		fmt.Println("load key error:", err)
		return err
	}
	conf.tlsConfig.Certificates = []tls.Certificate{cert}
	certPool := x509.NewCertPool()
	ca, err := os.Open(conf.CertPem)
	if err != nil {
		fmt.Println("open cert pem error:", err)
		return err
	}
	buf, err := ioutil.ReadAll(ca)
	if err != nil {
		fmt.Println("cert pem read error:", err)
		return err
	}
	if ok := certPool.AppendCertsFromPEM(buf); !ok {
		fmt.Println("append cert failed")
		return err
	}
	conf.tlsConfig.RootCAs = certPool

	if conf.err, err = NewLogger(conf.ErrorLog, "[dnsproxy]", "dnsproxy", syslog.LOG_ERR, log.LstdFlags); err != nil {
		return err
	}
	if conf.info, err = NewLogger(conf.InfoLog, "[dnsproxy]", "dnsproxy", syslog.LOG_INFO, log.LstdFlags); err != nil {
		return err
	}
	if conf.debug, err = NewLogger(conf.DebugLog, "[dnsproxy]", "dnsproxy", syslog.LOG_DEBUG, log.LstdFlags); err != nil {
		return err
	}
	return nil
}

func NewLogger(logger, prefix, tag string, priority syslog.Priority, flag int) (ret *log.Logger, err error) {
	var w io.Writer
	switch logger {
	case "":
		fallthrough
	case "stdout":
		w = os.Stdout
	case "stderr":
		w = os.Stderr
	case "syslog":
		w, err = syslog.New(priority, tag)
		if err != nil {
			return
		}
	default:
		w, err = os.OpenFile(logger, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			return
		}
	}
	ret = log.New(w, prefix, flag)
	return
}
