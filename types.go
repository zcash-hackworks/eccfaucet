package main

// GetBlockchainInfo return the zcashd rpc `getblockchaininfo` status
// https://zcash-rpc.github.io/getblockchaininfo.html
type GetBlockchainInfo struct {
	Chain                string     `json:"chain"`
	Blocks               int        `json:"blocks"`
	Headers              int        `json:"headers"`
	BestBlockhash        string     `json:"bestblockhash"`
	Difficulty           float64    `json:"difficulty"`
	VerificationProgress float64    `json:"verificationprogress"`
	SizeOnDisk           float64    `json:"size_on_disk"`
	SoftForks            []SoftFork `json:"softforks"`
}

type SoftFork struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type z_gettotalbalance struct {
	Transparent string
	Private     string
	Total       string
}

type WalletAddress struct {
	Address string
	Notes   []string
}
