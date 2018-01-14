package x

import (
	"github.com/ethereum/go-ethereum/crypto"
)

// An EpochHash holds the hash for the current epoch. It is generated by an
// Ethereum smart contract every time the a new epoch begins.
type EpochHash struct {
	Hash []byte
}

// A MinerHash holds the hash for a miner. It is generated by an Ethereum smart
// contract during the registration of the miner.
type MinerHash struct {
	Hash []byte
}

func Sort(epoch EpochHash, miners []MinerHash) []MinerHash {

	// Generate all hashes for all miners.
	hashes := make([][]byte, len(miners))
	for i, miner := range miners {
		hash := crypto.Keccak256(epoch.Hash, miner.Hash)
		hashes[i] = hash
	}

	// Sort the hashes.

	return nil
}
