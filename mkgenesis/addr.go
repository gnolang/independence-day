package main

import (
	"fmt"

	"github.com/gnolang/gno/pkgs/bech32"
)

// addrKey decodes a bech32 address and re-encodes it in canonical `g1…` form.
// It is the same function as allocate/process_consolidated.go's addrKey, and
// deliberately so: the two packages are both `package main`, so the helper
// cannot be shared, but the definition of "a valid address" must not fork
// between the stage that produces rows and the stage that merges them.
//
// The point is the *decode*. A shape check — `^g1[0-9a-z]{38}$`, which is what
// this package used to do everywhere — accepts a transposed character, because
// a transposition preserves the shape and only breaks the bech32 checksum. That
// is the realistic failure: counterparty addresses arrive by email and get
// pasted into a sheet, and two swapped characters produce a well-shaped address
// nobody controls.
//
// Round-tripping also normalises: an address given in another HRP (`cosmos1…`)
// comes back as the `g1…` form of the same 20 bytes, so a caller comparing the
// result against the input detects a non-canonical spelling as well as an
// invalid one.
func addrKey(address string) (string, error) {
	_, bz, err := bech32.Decode(address)
	if err != nil {
		return "", fmt.Errorf("address %q is not valid bech32: %w", address, err)
	}
	if len(bz) != 20 {
		return "", fmt.Errorf("address %q has %d bytes, expected 20", address, len(bz))
	}
	return bech32.Encode("g", bz)
}

// checkAddr requires addr to be valid bech32 AND already written in canonical
// `g1…` form. Balance sheets are read by consumers that do a plain string
// comparison, so an address that merely decodes to the right bytes is not
// enough — it has to be spelled the way everything else spells it.
func checkAddr(addr string) error {
	key, err := addrKey(addr)
	if err != nil {
		return err
	}
	if key != addr {
		return fmt.Errorf("address %q is not in canonical g1 form (want %q)", addr, key)
	}
	return nil
}
