package main

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
)

func zSendManyHTTPPost(r *http.Request) (opid string, err error) {
	fmt.Printf("zSendManyHTTPPost address: %s\n", r.FormValue("address"))
	switch {
	case r.FormValue("address") == "":
		fmt.Println("address blank case")
		return "", errors.New("Form field value required: address")
	case isTestnetTransparent(r.FormValue("address")):
		fmt.Println("Address is a transparent testnet address")

		return r.FormValue("address"), nil

	default:
		fmt.Println("address default case")
		return "", errors.New("A valid address is required")
	}
}

func isTestnetTransparent(addr string) bool {
	//TODO Check length and encoding
	matched, _ := regexp.MatchString(`^tm`, addr)
	return matched
}

func isTestnetSaplingZaddr(addr string) bool {
	//TODO Check length and encoding
	matched, _ := regexp.MatchString(`^ztestsapling`, addr)
	return matched
}

func checkSourceAddress(rAddress string) error {

	return nil
}
func checkFaucetAddress(checkAddr string) error {
	switch {
	case checkAddr == "":
		fmt.Println("address blank case")
		return errors.New("Form field value required: address")
	case isTestnetTransparent(checkAddr):
		fmt.Println("Address is a testnet transparent address")
		return nil
	case isTestnetSaplingZaddr(checkAddr):
		fmt.Println("Address is a testnet sapling address")
		return nil
	default:
		fmt.Println("address default case")
		return errors.New("A valid address is required")
	}
}

func zSendManyFaucet(addr string) (opid string, err error) {
	return "000-test-opid-string-000", nil
}
