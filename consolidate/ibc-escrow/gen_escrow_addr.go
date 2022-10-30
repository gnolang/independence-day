package main

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/gnolang/gno/pkgs/bech32"
)

func main() {
	maxChannelID := 426
	fileData := ""

	for i := 0; i < maxChannelID; i++ {
		channel := fmt.Sprintf("channel-%d", i)
		bz := GetEscrowBz("transfer", channel)
		cosmAddr := MustBech32Addr("cosmos", bz)
		gnoAddr := MustBech32Addr("g", bz)
		fileData += fmt.Sprintf("%s:%s:%s\n", cosmAddr, gnoAddr, channel)
	}

	err := os.WriteFile("ibc_escrow_address.txt", []byte(fileData), 0644)
	if err != nil {
		panic(err)
	}
	fmt.Println("File generated")
}

func GetEscrowBz(portID, channelID string) []byte {
	contents := fmt.Sprintf("%s/%s", portID, channelID)

	preImage := []byte("ics20-1")
	preImage = append(preImage, 0)
	preImage = append(preImage, contents...)
	hash := sha256.Sum256(preImage)

	return hash[:20]
}

func MustBech32Addr(prefix string, bz []byte) string {
	addr, err := bech32.Encode(prefix, bz)
	if err != nil {
		panic(err)
	}
	return addr
}
