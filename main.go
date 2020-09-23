package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gobuffalo/packr"
	"github.com/kelseyhightower/envconfig"
	"github.com/muesli/cache2go"
	"github.com/ybbus/jsonrpc"
	"github.com/zcash-hackworks/eccfaucet/pkg/eccfaucet"
	"github.com/zcash-hackworks/eccfaucet/pkg/rpc"
)

type ECCfaucetConfig struct {
	ListenPort          string
	ListenAddress       string
	RPCUser             string
	RPCPassword         string
	RPCHost             string
	RPCPort             string
	FundingAddress      string
	TLSCertFile         string
	TLSKeyFile          string
	TapAmount           float64
	TapWaitMinutes      float64
	OpStatusWaitSeconds int
}

func (c *ECCfaucetConfig) checkConfig() error {
	if c.ListenPort == "" {
		c.ListenPort = "3000"
	}
	if c.ListenAddress == "" {
		c.ListenPort = "127.0.0.1"
	}
	if c.RPCHost == "" {
		c.ListenPort = "localhost"
	}
	if c.ListenPort == "" {
		c.ListenPort = "3000"
	}
	if c.FundingAddress == "" {
		return fmt.Errorf("ECCFAUCET_FUNDINGADDRESS is required")
	}
	if (c.TLSCertFile == "" && c.TLSKeyFile != "") ||
		(c.TLSCertFile != "" && c.TLSKeyFile == "") {
		return fmt.Errorf("ECCFAUCET_TLSCERTFILE and ECCFAUCET_TLSKEYFILE are both required")
	}
	c.TapAmount = 1.0
	c.TapWaitMinutes = 2
	c.OpStatusWaitSeconds = 120
	return nil
}

func getBlockchainInfo(rpcClient jsonrpc.RPCClient) (blockChainInfo *rpc.GetBlockchainInfo, err error) {
	if err := rpcClient.CallFor(&blockChainInfo, "getblockchaininfo"); err != nil {
		return nil, err
	}
	return
}

func getInfo(rpcClient jsonrpc.RPCClient) (info *rpc.GetBlockInfo, err error) {
	if err := rpcClient.CallFor(&info, "getinfo"); err != nil {
		return nil, err
	}
	return info, nil
}

func main() {
	versionFlag := flag.Bool("version", false, "print version information")
	flag.Parse()
	if *versionFlag {
		fmt.Printf("(version=%s, branch=%s, gitcommit=%s)\n", Version, Branch, GitCommit)
		fmt.Printf("(go=%s, user=%s, date=%s)\n", GoVersion, BuildUser, BuildDate)
		os.Exit(0)
	}

	var zConfig ECCfaucetConfig
	err := envconfig.Process("eccfaucet", &zConfig)
	if err != nil {
		log.Fatal(err.Error())
	}
	if err = zConfig.checkConfig(); err != nil {
		log.Fatalf("Config error: %s", err)
	}
	fmt.Printf("zfaucet: %#v\n", zConfig)

	basicAuth := base64.StdEncoding.EncodeToString([]byte(zConfig.RPCUser + ":" + zConfig.RPCPassword))
	var z eccfaucet.ECCFaucet
	z.TapCache = cache2go.Cache("tapRequests")
	z.FundingAddress = zConfig.FundingAddress
	z.TapAmount = zConfig.TapAmount
	z.TapWaitMinutes = zConfig.TapWaitMinutes
	z.OpStatusWaitSeconds = zConfig.OpStatusWaitSeconds
	z.Operations = make(map[string]eccfaucet.OperationStatus)
	z.RPCConnetion = jsonrpc.NewClientWithOpts("http://"+zConfig.RPCHost+":"+zConfig.RPCPort,
		&jsonrpc.RPCClientOpts{
			CustomHeaders: map[string]string{
				"Authorization": "Basic " + basicAuth,
			}})

	go z.ClearCache()
	go z.UpdateZcashInfo()

	box := packr.NewBox("./templates")
	z.HomeHTML, err = box.FindString("eccfaucet.html")
	if err != nil {
		log.Fatal(err)
	}
	homeHandler := http.HandlerFunc(z.Home)
	balanceHandler := http.HandlerFunc(z.Balance)
	opsStatusHandler := http.HandlerFunc(z.OpsStatus)
	addressHandler := http.HandlerFunc(z.Addresses)
	mux := http.NewServeMux()
	mux.Handle("/", homeHandler)
	mux.Handle("/balance", z.OKMiddleware(balanceHandler))
	mux.Handle("/addresses", z.OKMiddleware(addressHandler))
	mux.Handle("/ops/status", z.OKMiddleware(opsStatusHandler))
	log.Printf("Listening on :%s...\n", zConfig.ListenPort)
	if zConfig.TLSCertFile != "" && zConfig.TLSKeyFile != "" {
		// https://gist.github.com/denji/12b3a568f092ab951456
		cfg := &tls.Config{
			MinVersion:               tls.VersionTLS12,
			CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
			PreferServerCipherSuites: true,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		}
		srv := &http.Server{
			Addr:         zConfig.ListenAddress + ":" + zConfig.ListenPort,
			Handler:      mux,
			TLSConfig:    cfg,
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		}
		err = srv.ListenAndServeTLS(zConfig.TLSCertFile, zConfig.TLSKeyFile)

	} else {
		err = http.ListenAndServe(zConfig.ListenAddress+":"+zConfig.ListenPort, mux)
	}
	log.Fatal(err)
}

// GetBytes returns a byte slice from an interface
func GetBytes(key interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(key)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil

}
