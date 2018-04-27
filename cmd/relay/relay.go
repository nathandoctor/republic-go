package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/republicprotocol/republic-go/blockchain/ethereum/hd"
	"github.com/republicprotocol/republic-go/blockchain/swap"

	"github.com/republicprotocol/republic-go/order"

	. "github.com/republicprotocol/republic-go/relay"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/republicprotocol/republic-go/blockchain/ethereum"
	"github.com/republicprotocol/republic-go/blockchain/ethereum/dnr"
	"github.com/republicprotocol/republic-go/blockchain/test/ganache"
	"github.com/republicprotocol/republic-go/crypto"
	"github.com/republicprotocol/republic-go/dispatch"
	"github.com/republicprotocol/republic-go/identity"
	"github.com/republicprotocol/republic-go/orderbook"
	"github.com/republicprotocol/republic-go/rpc/client"
	"github.com/republicprotocol/republic-go/rpc/dht"
	"github.com/republicprotocol/republic-go/rpc/relayer"
	"github.com/republicprotocol/republic-go/rpc/smpcer"
	"github.com/republicprotocol/republic-go/rpc/swarmer"
	"google.golang.org/grpc"
)

func main() {
	keystore := flag.String("keystore", "", "Encrypted keystore file")
	passphrase := flag.String("passphrase", "", "Passphrase for the encrypted keystore file")
	bind := flag.String("bind", "127.0.0.1", "Binding address for the gRPC and HTTP API")
	port := flag.String("port", "18515", "Binding port for the HTTP API")
	token := flag.String("token", "", "Bearer token for restricting access")
	configLocation := flag.String("config", "", "Relay configuration file location")
	maxConnections := flag.Int("maxConnections", 4, "Maximum number of connections to peers during synchronization")
	flag.Parse()

	fmt.Println("Decrypting keystore...")
	key, err := getKey(*keystore, *passphrase)
	if err != nil {
		fmt.Println(fmt.Errorf("cannot obtain key: %s", err))
		return
	}

	// keyPair, err := getKeyPair(key)
	// if err != nil {
	// 	fmt.Println(fmt.Errorf("cannot obtain keypair: %s", err))
	// 	return
	// }

	// multiAddr, err := getMultiaddress(keyPair, *port)
	// if err != nil {
	// 	fmt.Println(fmt.Errorf("cannot obtain multiaddress: %s", err))
	// 	return
	// }

	// Create gRPC server and TCP listener always using port 18514
	server := grpc.NewServer()
	listener, err := net.Listen("tcp", fmt.Sprintf("%v:18514", *bind))
	if err != nil {
		log.Fatal(err)
	}


	// Create Relay
	// config := Config{
	// 	KeyPair:      keyPair,
	// 	MultiAddress: multiAddr,
	// 	Token:        *token,
	// }

	config := LoadConfig(configLocation);

	registrar, err := getRegistry(config)
	if err != nil {
		fmt.Println(fmt.Errorf("cannot obtain registrar: %s", err))
		return
	}

	hyperdrive, err := getHyperdrive(config)
	if err != nil {
		fmt.Println(fmt.Errorf("cannot obtain hyperdrive: %s", err))
		return
	}

	book := orderbook.NewOrderbook(100)
	crypter := crypto.NewWeakCrypter()
	dht := dht.NewDHT(multiAddr.Address(), 100)
	connPool := client.NewConnPool(100)
	relayerClient := relayer.NewClient(&crypter, &dht, &connPool)
	smpcerClient := smpcer.NewClient(&crypter, multiAddr, &connPool)
	swarmerClient := swarmer.NewClient(&crypter, multiAddr, &dht, &connPool)
	relay := NewRelay(config, registrar, &book, &relayerClient, &smpcerClient, &swarmerClient)

	entries := make(chan orderbook.Entry)
	defer close(entries)
	go func() {
		defer book.Unsubscribe(entries)
		if err := book.Subscribe(entries); err != nil {
			log.Fatalf("cannot subscribe to orderbook: %v", err)
		}
	}()
	
	ethereumConn, err := ethereum.Connect("", ethereum.NetworkRopsten, config.)
	if err != nil {
		log.Fatalf("cannot connect to ethereum: %v", err)
	}
	
	confirmedOrders := processOrderbookEntries()
	swaps := executeConfirmedOrders()
	processAtomicSwaps(swaps)
	
	// Server gRPC and RESTful API
	fmt.Println(fmt.Sprintf("Relay API available at %s:%s", *bind, *port))
	dispatch.CoBegin(func() {
		if err := relay.ListenAndServe(*bind, *port); err != nil {
			log.Fatalf("error serving http: %v", err)
		}
	}, func() {
		relay.Register(server)
		if err := server.Serve(listener); err != nil {
			log.Fatalf("error serving grpc: %v", err)
		}
	}, func() {
		relay.Sync(context.Background(), *maxConnections)
	})

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
	go func() {
		<-sig
		server.Stop()
	}()

	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}

func processOrderbookEntries(hyperdrive hd.HyperdriveContract, entryInCh <-chan orderbook.Entry) <-chan orderbook.Entry {
	unconfirmedOrders := make(chan orderbook.Entry, 100)
	confirmedEntries := make(chan orderbook.Entry)

	orderConfirmed := func(orderID byte[32]) {
		depth, err := hyperdrive.GetDepth(orderID)
		if err != nil {
			log.Fatalf("failed to get depth: %v", err)
		}
		return (depth >= 16)
	}()

	go func() {
		defer close(confirmedEntries)
		for {
			select {
			case entry, ok := <-entryInCh:
				if !ok {
					return
				}
				if !orderConfirmed(entry.Order.ID) {
					unconfirmedOrders <- entry
				} else {
					entry.Status = order.Confirmed
					confirmedEntries <- entry
				}
			}
		}
	}()

	go func() {
		defer close(unconfirmedOrders)
		for {
			select {
			case entry, ok := <-unconfirmedOrders:
				if !ok {
					return
				}
				if !orderConfirmed(entry.Order.ID) {
					unconfirmedOrders <- entry
					time.Sleep(time.Second)
				} else {
					entry.Status = order.Confirmed
					confirmedEntries <- entry
				}
			}
		}
	}()
	return confirmedEntries
}

func executeConfirmedOrders(ctx context.Context, conn ethereum.Conn, auth *bind.TransactOpts, hyperdrive hd.HyperdriveContract, entries <-chan orderbook.Entry) <-chan swap.Swap {
	swaps := make(chan swap.Swap)
	
	go func() {
		defer close(swaps)
		for {
			select {
			case entry, ok := <-entries:
				if !ok {
					return
				}
				orderID := [32]byte{}
				copy(orderID[:], entry.Order.ID)
				_, orderIDs, err := hdc.GetOrderMatch(orderID)
				if err != nil {
					log.Fatalf("failed to get order match: %v", err)
					continue
				}
				if orderID == orderIDs[0] {
					swaps <- initSwap(ctx, conn, auth, entry, orderIDs[0], orderIDs[1])
				} else {
					swaps <- initSwap(ctx, conn, auth, entry, orderIDs[1], orderIDs[0])
				}
			}
		}
	}()

	return swaps, errs
}

func getKey(filename, passphrase string) (*keystore.Key, error) {
	// Read data from the keystore file and generate the key
	encryptedKey, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot read keystore file: %v", err)
	}

	key, err := keystore.DecryptKey(encryptedKey, passphrase)
	if err != nil {
		return nil, fmt.Errorf("cannot decrypt key with provided passphrase: %v", err)
	}

	return key, nil
}

func getKeyPair(key *keystore.Key) (identity.KeyPair, error) {
	id, err := identity.NewKeyPairFromPrivateKey(key.PrivateKey)
	if err != nil {
		return identity.KeyPair{}, fmt.Errorf("cannot generate id from key %v", err)
	}
	return id, nil
}

func getMultiaddress(id identity.KeyPair, port string) (identity.MultiAddress, error) {
	// Get our IP address
	ipInfoOut, err := exec.Command("curl", "https://ipinfo.io/ip").Output()
	if err != nil {
		return identity.MultiAddress{}, err
	}
	ipAddress := strings.Trim(string(ipInfoOut), "\n ")

	relayMultiaddress, err := identity.NewMultiAddressFromString(fmt.Sprintf("/ip4/%s/tcp/%s/republic/%s", ipAddress, port, id.Address().String()))
	if err != nil {
		return identity.MultiAddress{}, fmt.Errorf("cannot obtain trader multi address %v", err)
	}

	return relayMultiaddress, nil
}

func getRegistry(config *Config) (dnr.DarknodeRegistry, error) {
	conn, err := ethereum.Connect(config.Ethereum)
	auth := bind.NewKeyedTransactor(config.KeyPair.PrivateKey)
	if err != nil {
		fmt.Println(fmt.Errorf("cannot fetch dark node registry: %s", err))
		return dnr.DarknodeRegistry{}, err
	}
	auth.GasPrice = big.NewInt(6000000000)
	registrar, err := dnr.NewDarknodeRegistry(context.Background(), conn, auth, &bind.CallOpts{})
	if err != nil {
		fmt.Println(fmt.Errorf("cannot fetch dark node registry: %s", err))
		return dnr.DarknodeRegistry{}, err
	}
	return registrar, nil
}

func getHyperdrive(config *Config) (hd.HyperdriveContract, error) {
	conn, err := ethereum.Connect(config.Ethereum)
	auth := bind.NewKeyedTransactor(config.KeyPair.PrivateKey)
	if err != nil {
		fmt.Println(fmt.Errorf("cannot fetch hyperdrive: %s", err))
		return hd.HyperdriveContract{}, err
	}
	auth.GasPrice = big.NewInt(6000000000)
	hyperdrive, err := hd.NewHyperdriveContract(context.Background(), conn, auth, &bind.CallOpts{})
	if err != nil {
		fmt.Println(fmt.Errorf("cannot fetch hyperdrive: %s", err))
		return hd.HyperdriveContract{}, err
	}
	return hyperdrive, nil
}