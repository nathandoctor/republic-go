package status

import (
	"sync"

	"github.com/republicprotocol/republic-go/dht"
	"github.com/republicprotocol/republic-go/identity"
)

// Writer will write the address
type Writer interface {
	WriteNetwork(network string) error
	WriteMultiAddress(multiAddress identity.MultiAddress) error
	WriteEthereumAddress(ethAddress string) error
	WritePublicKey(publicKey []byte) error
}

// Reader the address
type Reader interface {
	Network() (string, error)
	MultiAddress() (identity.MultiAddress, error)
	EthereumAddress() (string, error)
	PublicKey() ([]byte, error)
	Peers() (int, error)
}

/*

Basic information
* network
* multiaddress
* ethereum address
* connected peers
* basic funds amounts, balance, e.g. eth
* fees it's earned
* and register or deregister a dark node (deregister is disabled metamask address is not the owner)

*/

type Provider interface {
	Writer
	Reader
}

type provider struct {
	mu                      *sync.Mutex
	dht                     *dht.DHT
	network                 string
	multiAddress            identity.MultiAddress
	ethereumAddress         string
	darknodeRegistryAddress string
	orderbookAddress        string
	rewardVaultAddress      string
	publicKey               []byte
}

// NewProvider returns a new provider
func NewProvider(dht *dht.DHT) Provider {
	return &provider{
		mu:  new(sync.Mutex),
		dht: dht,
	}
}

// WriteNetwork writes network to the provider
func (sp *provider) WriteNetwork(network string) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.network = network
	return nil
}

// Network reads the provider for the network
func (sp *provider) Network() (string, error) {
	return sp.network, nil
}

func (sp *provider) WriteMultiAddress(multiAddr identity.MultiAddress) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.multiAddress = multiAddr
	return nil
}

func (sp *provider) MultiAddress() (identity.MultiAddress, error) {
	return sp.multiAddress, nil
}

// WriteEthereumAddress writes ethAddr to the provider
func (sp *provider) WriteEthereumAddress(ethAddr string) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.ethereumAddress = ethAddr
	return nil
}

// EthereumAddress gets the ethereum address
func (sp *provider) EthereumAddress() (string, error) {
	return sp.ethereumAddress, nil
}

// WriteDarknodeRegistryAddress writes darknodeRegistryAddress to the provider
func (sp *provider) WriteDarknodeRegistryAddress(darknodeRegistryAddress string) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.darknodeRegistryAddress = darknodeRegistryAddress
	return nil
}

// DarknodeRegistryAddress gets the DarknodeRegistry contract address
func (sp *provider) DarknodeRegistryAddress() (string, error) {
	return sp.darknodeRegistryAddress, nil
}

// WriteOrderbookAddress writes orderbookAddress to the provider
func (sp *provider) WriteOrderbookAddress(orderbookAddress string) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.orderbookAddress = orderbookAddress
	return nil
}

// OrderbookAddress gets the Orderbook contract address
func (sp *provider) OrderbookAddress() (string, error) {
	return sp.ethereumAddress, nil
}

// WriteRewardVaultAddress writes rewardVaultAddress to the provider
func (sp *provider) WriteRewardVaultAddress(rewardVaultAddress string) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.rewardVaultAddress = rewardVaultAddress
	return nil
}

// RewardVaultAddress gets the RewardVault contract address
func (sp *provider) RewardVaultAddress() (string, error) {
	return sp.rewardVaultAddress, nil
}

// WritePublicKey writes the dark node's public key to the provider
func (sp *provider) WritePublicKey(publicKey []byte) error {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.publicKey = publicKey
	return nil
}

// PublicKey gets the public key
func (sp *provider) PublicKey() ([]byte, error) {
	return sp.publicKey, nil
}

// Peers returns the number of peers the darknode is connected to
func (sp *provider) Peers() (int, error) {
	return len(sp.dht.MultiAddresses()), nil
}
