package main

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gobuffalo/packr"
	"github.com/kelseyhightower/envconfig"
	_ "github.com/lib/pq"
	"github.com/muesli/cache2go"
	"github.com/ybbus/jsonrpc"
)

const tapAmount = 1.0
const tapWaitMinutes = 2
const opStatusWaitSeconds = 120

type TapRequest struct {
	NetworkAddress string
	WalletAddress  string
	RequestedAt    time.Time
}
type ECCfaucetConfig struct {
	ListenPort     string
	ListenAddress  string
	RPCUser        string
	RPCPassword    string
	RPCHost        string
	RPCPort        string
	FundingAddress string
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
	return nil
}

// ECCFaucet holds a zfaucet configuration
type ECCFaucet struct {
	RPCConnetion     jsonrpc.RPCClient
	CurrentHeight    int
	UpdatedChainInfo time.Time
	UpdatedWallet    time.Time
	Operations       map[string]OperationStatus
	ZcashdVersion    string
	ZcashNetwork     string
	FundingAddress   string
	TapRequests      []*TapRequest
	TapCache         *cache2go.CacheTable
	HomeHTML         string
}

type SendAmount struct {
	Address string  `json:"address"`
	Amount  float32 `json:"amount"`
}

// TODO tag facet transactions, zaddr targets only
type SendAmountMemo struct {
	SendAmount
	Memo string
}

func (z *ECCFaucet) ClearCache() {
	for {
		now := time.Now()
		fmt.Printf("Clearing cache: %d\n", len(z.TapRequests))
		for _, t := range z.TapRequests {
			fmt.Printf("Checking RemoteAddress: '%#v' - '%#v'\n", t.NetworkAddress, t.RequestedAt)
			diff := now.Sub(t.RequestedAt)
			if diff.Minutes() > tapWaitMinutes {
				fmt.Printf("Old entry! : %#v\n", t)
			}
		}
		time.Sleep(time.Second * 60 * tapWaitMinutes)
	}
}

func (z *ECCFaucet) UpdateZcashInfo() {
	for {
		z.UpdatedChainInfo = time.Now()
		zChainInfo, err := getBlockchainInfo(z.RPCConnetion)
		if err != nil {
			fmt.Printf("Failed to get blockchaininfo: %s\n", err)
		} else {
			z.CurrentHeight = zChainInfo.Blocks
			z.ZcashNetwork = zChainInfo.Chain
		}
		zVersion, err := getInfo(z.RPCConnetion)
		if err != nil {
			fmt.Printf("Failed to getinfo: %s\n", err)
		} else {
			z.ZcashdVersion = strconv.Itoa(zVersion.Version)
		}
		fmt.Println("Updated Zcashd Info")
		time.Sleep(time.Second * 30)
	}
}

func (z *ECCFaucet) WaitForOperation(opid string) (os OperationStatus, err error) {
	var opStatus []struct {
		CreationTime int    `json:"creation_time"`
		ID           string `json:"id"`
		Method       string `json:"method"`
		Result       struct {
			TxID string `json:"txid"`
		}
		Status string `json:"status"`
	}
	var parentList [][]string
	var opList []string
	opList = append(opList, opid)
	parentList = append(parentList, opList)
	fmt.Printf("opList: %s\n", opList)
	fmt.Printf("parentList: %s\n", parentList)
	// Wait for a few seconds for the operational status to become available
	for i := 0; i < opStatusWaitSeconds; i++ {
		if err := z.RPCConnetion.CallFor(
			&opStatus,
			"z_getoperationresult",
			parentList,
		); err != nil {
			return os, fmt.Errorf("failed to call z_getoperationresult: %s", err)
		} else {
			fmt.Printf("op: %s, i: %d, status: %#v\n", opid, i, opStatus)
			if len(opStatus) > 0 {
				fmt.Printf("opStatus: %#v\n", opStatus[0])
				//z.Operations[opid] = OperationStatus{
				os = OperationStatus{
					UpdatedAt: time.Now(),
					TxID:      opStatus[0].Result.TxID,
					Status:    opStatus[0].Status,
				}
				z.Operations[opid] = os
				return os, nil
			}
		}
		time.Sleep(time.Second * 1)
	}
	return os, errors.New("Timeout waiting for operations status")
}

func (z *ECCFaucet) ValidateFundingAddress() (bool, error) {
	if z.FundingAddress == "" {
		return false, errors.New("FundingAddressis required")
	}
	return true, nil
}

func (z *ECCFaucet) ZSendManyFaucet(remoteAddr string, remoteWallet string) (opStatus OperationStatus, err error) {
	var op *string
	amountEntry := SendAmount{
		Address: remoteWallet,
		Amount:  tapAmount,
	}
	fmt.Printf("ZSendManyFaucet sending: %#v\n", amountEntry)
	fmt.Printf("ZSendManyFaucet from funding address: %s\n", z.FundingAddress)
	// if err != nil {
	// 	return opStatus, err
	// }
	// Call z_sendmany with a single entry entry list
	if err := z.RPCConnetion.CallFor(
		&op,
		"z_sendmany",
		z.FundingAddress,
		[]SendAmount{amountEntry},
	); err != nil {
		return opStatus, err
	}
	fmt.Printf("ZSendManyFaucet sent to %s: Address: %s %s\n", remoteWallet, remoteAddr, *op)
	opStatus, err = z.WaitForOperation(*op)
	if err != nil {
		return opStatus, err
	}
	if opStatus.Status != "success" {
		return opStatus, fmt.Errorf("Failed to send funds: %s", err)
	}
	tapRequest := &TapRequest{
		NetworkAddress: remoteAddr,
		WalletAddress:  remoteWallet,
		RequestedAt:    time.Now(),
	}
	z.TapCache.Add(remoteAddr, tapWaitMinutes*60*time.Second, tapRequest)
	z.TapRequests = append(z.TapRequests, tapRequest)
	return opStatus, err

}

type GetBlockInfo struct {
	Version int
}

func getBlockchainInfo(rpcClient jsonrpc.RPCClient) (blockChainInfo *GetBlockchainInfo, err error) {
	if err := rpcClient.CallFor(&blockChainInfo, "getblockchaininfo"); err != nil {
		return nil, err
	}
	return
}

func getInfo(rpcClient jsonrpc.RPCClient) (info *GetBlockInfo, err error) {
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
	err := envconfig.Process("zfaucet", &zConfig)
	if err != nil {
		log.Fatal(err.Error())
	}
	if err = zConfig.checkConfig(); err != nil {
		log.Fatalf("Config error: %s", err)
	}
	fmt.Printf("zfaucet: %#v\n", zConfig)

	basicAuth := base64.StdEncoding.EncodeToString([]byte(zConfig.RPCUser + ":" + zConfig.RPCPassword))
	var z ECCFaucet
	z.TapCache = cache2go.Cache("tapRequests")
	z.FundingAddress = zConfig.FundingAddress
	z.Operations = make(map[string]OperationStatus)
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
	homeHandler := http.HandlerFunc(z.home)
	balanceHandler := http.HandlerFunc(z.balance)
	opsStatusHandler := http.HandlerFunc(z.opsStatus)
	addressHandler := http.HandlerFunc(z.addresses)
	mux := http.NewServeMux()
	mux.Handle("/", homeHandler)
	mux.Handle("/balance", z.OKMiddleware(balanceHandler))
	mux.Handle("/addresses", z.OKMiddleware(addressHandler))
	mux.Handle("/ops/status", z.OKMiddleware(opsStatusHandler))
	log.Printf("Listening on :%s...\n", zConfig.ListenPort)
	err = http.ListenAndServe(zConfig.ListenAddress+":"+zConfig.ListenPort, mux)
	log.Fatal(err)
}

// OperationStatus describes an rpc response
type OperationStatus struct {
	UpdatedAt time.Time
	Status    string
	TxID      string
	result    interface{}
}

// home is the default request handler
func (z *ECCFaucet) home(w http.ResponseWriter, r *http.Request) {
	// tData is the html template data
	tData := struct {
		Z   *ECCFaucet
		Msg string
	}{
		z,
		"",
	}
	switch r.Method {
	case http.MethodPost:
		res, err := z.TapCache.Value(r.RemoteAddr)
		if err == nil {
			fmt.Println("Found value in cache:", res.Data().(*TapRequest).NetworkAddress)
			tData.Msg = fmt.Sprintf("You may only tap the faucet every %d minutes\nPlease try again later\n", tapWaitMinutes)
			break
		} else {
			fmt.Println("Error retrieving value from cache:", err)
		}
		if err := checkFaucetAddress(r.FormValue("address")); err != nil {
			tData.Msg = fmt.Sprintf("Invalid address: %s", err)
			break
		}
		opStatus, err := z.ZSendManyFaucet(r.RemoteAddr, r.FormValue("address"))
		if err != nil {
			tData.Msg = fmt.Sprintf("Failed to send funds: %s", err)
			break
		}
		tData.Msg = fmt.Sprintf("Successfully submitted operation, transaction: %s", opStatus.TxID)
	}
	w.Header().Set("Content-Type", "text/html")
	tmpl, err := template.New("name").Parse(z.HomeHTML)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	tmpl.Execute(w, tData)
}

// OKMiddleware determines if a request is allowed before execution
func (z *ECCFaucet) OKMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Our middleware logic goes here...
		next.ServeHTTP(w, r)
	})
}

// Balance
func (z *ECCFaucet) balance(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var totalBalance *z_gettotalbalance
	if err := z.RPCConnetion.CallFor(&totalBalance, "z_gettotalbalance"); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	out, err := json.Marshal(totalBalance)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	fmt.Fprintf(w, string(out))
}

// opsStatus
func (z *ECCFaucet) opsStatus(w http.ResponseWriter, r *http.Request) {
	// tData is the html template data
	tData := struct {
		Z    *ECCFaucet
		Ops  *[]string
		Type string
	}{
		z,
		nil,
		"opsStatus",
	}
	if err := z.RPCConnetion.CallFor(&tData.Ops, "z_listoperationids"); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	tmpl, err := template.New("name").Parse(z.HomeHTML)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	tmpl.Execute(w, tData)
}

// addresses
func (z *ECCFaucet) addresses(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var addresses []WalletAddress
	var zlist *[]string
	var taddrs []interface{}
	// Z addresses
	if err := z.RPCConnetion.CallFor(&zlist, "z_listaddresses"); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	for _, zaddr := range *zlist {
		entry := WalletAddress{
			Address: zaddr,
		}
		entry.Notes = append(entry.Notes, "z address")
		addresses = append(addresses, entry)

	}
	// T addresses
	if err := z.RPCConnetion.CallFor(&taddrs, "listaddressgroupings"); err != nil {
		http.Error(w, fmt.Sprintf("Problem calling listaddressgroupings: %s", err.Error()), 500)
		return
	}
	fmt.Printf("T addresses:\n%#v\n", taddrs)
	// TODO: fix this mess
	for _, a := range taddrs {
		switch aResult := a.(type) {
		case []interface{}:
			for _, b := range aResult {
				switch bResult := b.(type) {
				case []interface{}:
					for _, x := range bResult {
						switch x.(type) {
						case string:
							taddr := fmt.Sprintf("%v", x)
							fmt.Printf("Adding T Address: %s\n", taddr)
							entry := WalletAddress{
								Address: taddr,
							}
							entry.Notes = append(entry.Notes, "t address")
							addresses = append(addresses, entry)
						}
					}
				}
			}
		}
	}
	out, err := json.Marshal(addresses)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	fmt.Fprintf(w, string(out))
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
