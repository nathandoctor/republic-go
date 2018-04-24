package darknode

import (
	"context"
	"fmt"
	"time"

	"github.com/republicprotocol/republic-go/delta"
	"github.com/republicprotocol/republic-go/dispatch"
	"github.com/republicprotocol/republic-go/rpc"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/republicprotocol/republic-go/crypto"
	"github.com/republicprotocol/republic-go/darkocean"
	ethclient "github.com/republicprotocol/republic-go/ethereum/client"
	"github.com/republicprotocol/republic-go/ethereum/contracts"
	"github.com/republicprotocol/republic-go/identity"
	"github.com/republicprotocol/republic-go/logger"
	"github.com/republicprotocol/republic-go/orderbook"
	"github.com/republicprotocol/republic-go/relay"
	"github.com/republicprotocol/republic-go/smpc"
)

// Darknodes is an alias.
type Darknodes []Darknode

type Darknode struct {
	Config
	Logger *logger.Logger

	id           identity.ID
	address      identity.Address
	multiAddress identity.MultiAddress
	orderbook    orderbook.Orderbook
	crypter      crypto.Crypter

	darknodeRegistry contracts.DarkNodeRegistry

	rpc *rpc.RPC

	smpc  smpc.Smpc
	relay relay.Relay
}

// NewDarknode returns a new Darknode.
func NewDarknode(config Config) (Darknode, error) {
	node := Darknode{
		Config: config,
		Logger: logger.StdoutLogger,
	}

	// Get identity information from the Config
	key, err := identity.NewKeyPairFromPrivateKey(node.Config.EcdsaKey.PrivateKey)
	if err != nil {
		return node, fmt.Errorf("cannot get ID from private key: %v", err)
	}
	node.id = key.ID()
	node.address = key.Address()
	node.multiAddress = config.MultiAddress
	node.orderbook = orderbook.NewOrderbook(4)

	// Open a connection to the Ethereum network
	transactOpts := bind.NewKeyedTransactor(config.EcdsaKey.PrivateKey)
	ethclient, err := ethclient.Connect(
		config.Ethereum.URI,
		config.Ethereum.Network,
		config.Ethereum.RepublicTokenAddress,
		config.Ethereum.DarknodeRegistryAddress,
	)
	if err != nil {
		return node, err
	}

	// Create bindings to the DarknodeRegistry and Ocean
	darknodeRegistry, err := contracts.NewDarkNodeRegistry(context.Background(), ethclient, transactOpts, &bind.CallOpts{})
	if err != nil {
		return Darknode{}, err
	}
	node.darknodeRegistry = darknodeRegistry

	// FIXME: Use a production Crypter implementation
	weakCrypter := crypto.NewWeakCrypter()
	node.crypter = &weakCrypter

	node.rpc = rpc.NewRPC(node.crypter, node.multiAddress, &node.orderbook)

	node.relay = relay.NewRelay(relay.Config{}, darkocean.Pools{}, darknodeRegistry, &node.orderbook, node.rpc.RelayerClient(), node.rpc.SmpcerClient(), node.rpc.SwarmerClient())

	return node, nil
}

// Bootstrap the Darknode into the swarm network. The Darknode will query all
// reachable nodes for itself, updating its dht.DHT as it connects to other
// nodes. Calls to Darknode.Bootstrap are not blocking, and return a channel of
// errors encountered. Users should not call Darknode.Bootstrap until the
// Darknode is registered, and the its registration is approved.
func (node *Darknode) Bootstrap(ctx context.Context) <-chan error {
	return node.rpc.SwarmerClient().Bootstrap(ctx, node.BootstrapMultiAddresses, -1)
}

// Run the Darknode until the done channel is closed.
func (node *Darknode) Run(done <-chan struct{}) <-chan error {
	errs := make(chan error, 1)

	go func() {
		defer close(errs)

		// Wait until registration is approved
		if err := node.darknodeRegistry.WaitUntilRegistration(node.ID()[:]); err != nil {
			errs <- err
			return
		}

		// Bootstrap into the network
		time.Sleep(time.Second)

		dispatch.Pipe(done, node.RunEpochs(done), errs)
	}()

	return errs
}

// RunEpochs will watch for changes to the Ocean and run the secure
// multi-party computation with new Pools. Stops when the done channel is
// closed, and will attempt to recover from errors encountered while
// interacting with the Ocean.
func (node *Darknode) RunEpochs(done <-chan struct{}) <-chan error {
	errs := make(chan error, 1)

	go func() {
		err := node.darknodeRegistry.WaitUntilRegistration(node.ID())
		if err != nil {
			errs <- err
			return
		}

		// Maintain multiple done channels so that multiple epochs can be running
		// in parallel
		var prevDone chan struct{}
		var currDone chan struct{}
		defer func() {
			if prevDone != nil {
				close(prevDone)
			}
			if currDone != nil {
				close(currDone)
			}
		}()

		// Looping until the done channel is closed will recover from errors
		// returned by watching the Ocean
		for {
			select {
			case <-done:
				return
			default:
			}

			// Start watching epochs
			epochs, epochErrs := RunEpochWatcher(done, node.darknodeRegistry)
			go dispatch.Pipe(done, epochErrs, errs)

			for quit := false; !quit; {
				select {

				case <-done:
					return

				case err, ok := <-errs:
					if !ok {
						quit = true
						break
					}
					node.Logger.Network(logger.Error, err.Error())

				case epoch, ok := <-epochs:
					if !ok {
						quit = true
						break
					}
					if prevDone != nil {
						close(prevDone)
					}
					prevDone = currDone
					currDone = make(chan struct{})

					darknodeIDs, err := node.darknodeRegistry.GetAllNodes()
					if err != nil {
						// FIXME: Do not skip the epoch. Retry with a backoff.
						errs <- err
						continue
					}

					darkOcean := NewDarkOcean(epoch.Blockhash, darknodeIDs)
					deltas, deltaErrs := RunEpochProcess(currDone, node.ID(), darkOcean, node.router)
					go dispatch.Pipe(done, deltaErrs, errs)
					go func() {
						for delta := range deltas {
							if delta.IsMatch(smpc.Prime) {
								node.Logger.OrderMatch(logger.Info, delta.ID.String(), delta.BuyOrderID.String(), delta.SellOrderID.String())
							}
						}
					}()
				}
			}
		}
	}()

	return errs
}

// OrderMatchToHyperdrive converts an order match into a hyperdrive.Tx and
// forwards it to the Hyperdrive.
func (node *Darknode) OrderMatchToHyperdrive(delta delta.Delta) {
	if !delta.IsMatch(smpc.Prime) {
		return
	}
	// TODO: Implement
}

// ID returns the ID of the Darknode.
func (node *Darknode) ID() identity.ID {
	return node.id
}

// Address returns the Address of the Darknode.
func (node *Darknode) Address() identity.Address {
	return node.address
}

// MultiAddress returns the MultiAddress of the Darknode.
func (node *Darknode) MultiAddress() identity.MultiAddress {
	return node.multiAddress
}
