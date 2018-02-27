package dnr

import (
	"context"
	"errors"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/republicprotocol/go-dark-node-registrar/contracts"
)

// DarkNodeRegistrar is the dark node interface
type DarkNodeRegistrar struct {
	context                  context.Context
	client                   *Client
	auth1                    *bind.TransactOpts
	auth2                    *bind.CallOpts
	binding                  *contracts.DarkNodeRegistrar
	tokenBinding             *contracts.ERC20
	darkNodeRegistrarAddress common.Address
}

// NewDarkNodeRegistrar returns a Dark node registrar
func NewDarkNodeRegistrar(context context.Context, client *Client, auth1 *bind.TransactOpts, auth2 *bind.CallOpts, address common.Address, renAddress common.Address, data []byte) *DarkNodeRegistrar {
	contract, err := contracts.NewDarkNodeRegistrar(address, bind.ContractBackend(*client))
	if err != nil {
		log.Fatalf("%v", err)
	}
	renContract, err := contracts.NewERC20(renAddress, bind.ContractBackend(*client))
	if err != nil {
		log.Fatalf("%v", err)
	}
	return &DarkNodeRegistrar{
		context:                  context,
		client:                   client,
		auth1:                    auth1,
		auth2:                    auth2,
		binding:                  contract,
		tokenBinding:             renContract,
		darkNodeRegistrarAddress: address,
	}
}

// Register registers a new dark node
func (darkNodeRegistrar *DarkNodeRegistrar) Register(_darkNodeID []byte, _publicKey []byte) (*types.Transaction, error) {
	value, err := darkNodeRegistrar.binding.MinimumBond(darkNodeRegistrar.auth2)
	if err != nil {
		return &types.Transaction{}, err
	}
	balance, err := darkNodeRegistrar.tokenBinding.BalanceOf(darkNodeRegistrar.auth2, darkNodeRegistrar.auth1.From)
	if err != nil {
		return &types.Transaction{}, err
	}
	if balance.Cmp(value) < 0 {
		return &types.Transaction{}, errors.New("Not enough balance to register a node")
	}
	tx, err := darkNodeRegistrar.tokenBinding.Approve(darkNodeRegistrar.auth1, darkNodeRegistrar.darkNodeRegistrarAddress, value)
	if err != nil {
		return tx, err
	}
	_, err = PatchedWaitMined(darkNodeRegistrar.context, *darkNodeRegistrar.client, tx)
	if err != nil {
		return tx, err
	}
	_darkNodeIDByte, err := toByte(_darkNodeID)
	if err != nil {
		return &types.Transaction{}, err
	}

	txn, err := darkNodeRegistrar.binding.Register(darkNodeRegistrar.auth1, _darkNodeIDByte, _publicKey)
	if err == nil {
		_, err := PatchedWaitMined(darkNodeRegistrar.context, *darkNodeRegistrar.client, txn)
		if err != nil {
			return txn, err
		}
	}
	return txn, err
}

// Deregister deregisters an existing dark node
func (darkNodeRegistrar *DarkNodeRegistrar) Deregister(_darkNodeID []byte) (*types.Transaction, error) {
	_darkNodeIDByte, err := toByte(_darkNodeID)
	if err != nil {
		return &types.Transaction{}, err
	}
	return darkNodeRegistrar.binding.Deregister(darkNodeRegistrar.auth1, _darkNodeIDByte)
}

// GetBond get's the bond of an existing dark node
func (darkNodeRegistrar *DarkNodeRegistrar) GetBond(_darkNodeID []byte) (*big.Int, error) {
	_darkNodeIDByte, err := toByte(_darkNodeID)
	if err != nil {
		return &big.Int{}, err
	}
	return darkNodeRegistrar.binding.GetBond(darkNodeRegistrar.auth2, _darkNodeIDByte)
}

// IsDarkNodeRegistered check's whether a dark node is registered or not
func (darkNodeRegistrar *DarkNodeRegistrar) IsDarkNodeRegistered(_darkNodeID []byte) (bool, error) {
	_darkNodeIDByte, err := toByte(_darkNodeID)
	if err != nil {
		return false, err
	}
	return darkNodeRegistrar.binding.IsDarkNodeRegistered(darkNodeRegistrar.auth2, _darkNodeIDByte)
}

// CurrentEpoch returns the current epoch
func (darkNodeRegistrar *DarkNodeRegistrar) CurrentEpoch() (struct {
	Blockhash [32]byte
	Timestamp *big.Int
}, error) {
	return darkNodeRegistrar.binding.CurrentEpoch(darkNodeRegistrar.auth2)
}

// Epoch updates the current Epoch
func (darkNodeRegistrar *DarkNodeRegistrar) Epoch() (*types.Transaction, error) {
	return darkNodeRegistrar.binding.Epoch(darkNodeRegistrar.auth1)
}

// GetCommitment get's the signed commitment
func (darkNodeRegistrar *DarkNodeRegistrar) GetCommitment(_darkNodeID []byte) ([32]byte, error) {
	_darkNodeIDByte, err := toByte(_darkNodeID)
	if err != nil {
		return [32]byte{}, err
	}
	return darkNodeRegistrar.binding.GetCommitment(darkNodeRegistrar.auth2, _darkNodeIDByte)
}

// GetOwner get's the owner of the given dark node
func (darkNodeRegistrar *DarkNodeRegistrar) GetOwner(_darkNodeID []byte) (common.Address, error) {
	_darkNodeIDByte, err := toByte(_darkNodeID)
	if err != nil {
		return common.Address{}, err
	}
	return darkNodeRegistrar.binding.GetOwner(darkNodeRegistrar.auth2, _darkNodeIDByte)
}

// GetPublicKey get's the public key of the goven dark node
func (darkNodeRegistrar *DarkNodeRegistrar) GetPublicKey(_darkNodeID []byte) ([]byte, error) {
	_darkNodeIDByte, err := toByte(_darkNodeID)
	if err != nil {
		return []byte{}, err
	}
	return darkNodeRegistrar.binding.GetPublicKey(darkNodeRegistrar.auth2, _darkNodeIDByte)
}

// GetDarkpool get's the dark pool configuration
func (darkNodeRegistrar *DarkNodeRegistrar) GetDarkpool() ([][20]byte, error) {
	return darkNodeRegistrar.binding.GetXingOverlay(darkNodeRegistrar.auth2)
}

// MinimumBond get's the minimum viable bonda mount
func (darkNodeRegistrar *DarkNodeRegistrar) MinimumBond() (*big.Int, error) {
	return darkNodeRegistrar.binding.MinimumBond(darkNodeRegistrar.auth2)
}

// MinimumEpochInterval get's the minimum epoch interval
func (darkNodeRegistrar *DarkNodeRegistrar) MinimumEpochInterval() (*big.Int, error) {
	return darkNodeRegistrar.binding.MinimumEpochInterval(darkNodeRegistrar.auth2)
}

// PendingRefunds get's the pending refund amount of the given address
func (darkNodeRegistrar *DarkNodeRegistrar) PendingRefunds(arg0 common.Address) (*big.Int, error) {
	return darkNodeRegistrar.binding.PendingRefunds(darkNodeRegistrar.auth2, arg0)
}

// Refund refunds the bond of an unregistered miner
func (darkNodeRegistrar *DarkNodeRegistrar) Refund() (*types.Transaction, error) {
	return darkNodeRegistrar.binding.Refund(darkNodeRegistrar.auth1)
}

// WaitTillRegistration waits until the registration is successful.
func (darkNodeRegistrar *DarkNodeRegistrar) WaitTillRegistration(_darkNodeID []byte) error {
	isRegistered := false
	for !isRegistered {
		tx, err := darkNodeRegistrar.Epoch()
		if err != nil {

			return err
		}
		_, err = PatchedWaitMined(darkNodeRegistrar.context, *darkNodeRegistrar.client, tx)
		if err != nil {

			return err
		}
		time.Sleep(time.Minute)
		isRegistered, err = darkNodeRegistrar.IsDarkNodeRegistered(_darkNodeID)
		if err != nil {

			return err
		}
	}
	return nil
}

func toByte(id []byte) ([20]byte, error) {
	twentyByte := [20]byte{}
	if len(id) != 20 {
		return twentyByte, errors.New("Length mismatch")
	}
	for i := range id {
		twentyByte[i] = id[i]
	}
	return twentyByte, nil
}
