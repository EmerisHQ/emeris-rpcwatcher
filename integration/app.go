package integration

import (
	"encoding/json"

	ibctesting "github.com/allinbits/emeris-rpcwatcher/testing"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/simapp"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	capabilitykeeper "github.com/cosmos/cosmos-sdk/x/capability/keeper"
	ibckeeper "github.com/cosmos/cosmos-sdk/x/ibc/core/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	gaia "github.com/cosmos/gaia/v5/app"
	"github.com/tendermint/tendermint/libs/log"
	dbm "github.com/tendermint/tm-db"
)

type GaiaApp struct {
	*gaia.GaiaApp
}

// TestingApp functions

// GetBaseApp implements the TestingApp interface.
func (app GaiaApp) GetBaseApp() *baseapp.BaseApp {
	return app.BaseApp
}

// GetBankKeeper implements the TestingApp interface.
func (app GaiaApp) GetBankKeeper() bankkeeper.Keeper {
	return app.BankKeeper
}

// GetStakingKeeper implements the TestingApp interface.
func (app GaiaApp) GetStakingKeeper() stakingkeeper.Keeper {
	return app.StakingKeeper
}

// GetIBCKeeper implements the TestingApp interface.
func (app GaiaApp) GetIBCKeeper() *ibckeeper.Keeper {
	return app.IBCKeeper
}

// GetScopedIBCKeeper implements the TestingApp interface.
func (app GaiaApp) GetScopedIBCKeeper() capabilitykeeper.ScopedKeeper {
	return app.ScopedIBCKeeper
}

// GetScopedTransferKeeper implements the TestingApp interface.
func (app GaiaApp) GetScopedTransferKeeper() capabilitykeeper.ScopedKeeper {
	return app.ScopedTransferKeeper
}

// GetTxConfig implements the TestingApp interface.
func (app GaiaApp) GetTxConfig() client.TxConfig {
	return gaia.MakeEncodingConfig().TxConfig
}

func SetupTestingGaiaApp() (ibctesting.TestingApp, map[string]json.RawMessage) {
	db := dbm.NewMemDB()
	encCdc := gaia.MakeEncodingConfig()
	gaiaApp := gaia.NewGaiaApp(log.NewNopLogger(), db, nil, true, map[int64]bool{}, gaia.DefaultNodeHome, 5, encCdc, simapp.EmptyAppOptions{})
	app := GaiaApp{gaiaApp}
	return app, gaia.NewDefaultGenesisState()
}
