package integration

import (
	"fmt"

	"github.com/allinbits/emeris-rpcwatcher/rpcwatcher/database"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func getInsertQueryValue(index string, chain testChain) string {
	value := `(` + index + `, true, '` + chain.chainID + `', '10s', 'logo url', '` + chain.chainID + `', '{`
	i := 0
	for k, v := range chain.channels {
		value = value + `"` + k + `": "` + v + `"`
		if i != len(chain.channels)-1 {
			value += `,`
		}
		i++
	}
	value = value + `}',
	'[
		{"display_name":"` + chain.accountInfo.displayDenom + `","name":"` + chain.accountInfo.denom + `","verified":true,"fetch_price":true,"fee_token":true,"fee_levels":{"low":1,"average":22,"high":42},"precision":6}
	]', 
	ARRAY['feeaddress'], 'genesis_hash', 
	'{"endpoint":"http://localhost:` + chain.rpcPort + `","chain_id":"` + chain.chainID + `","bech32_config":{"main_prefix":"main_prefix","prefix_account":"` + chain.accountInfo.prefix + `","prefix_validator":"prefix_validator",
	"prefix_consensus":"prefix_consensus","prefix_public":"prefix_public","prefix_operator":"prefix_operator"}}', 
	'm/44''/118''/0''/0/0')`
	return value
}

func getMigrations(chains []testChain) []string {
	var migrations []string
	migrations = append(migrations, database.CreateDB, database.CreateCNSTable)
	insertQuery := `INSERT INTO cns.chains 
	(
	id, enabled, chain_name, valid_block_thresh, logo, display_name, primary_channel, denoms, demeris_addresses, 
	genesis_hash, node_info, derivation_path
	) 
	VALUES `
	for i, chain := range chains {
		insertQuery += getInsertQueryValue(fmt.Sprintf("%d", i+1), chain)
		if i != len(chains)-1 {
			insertQuery += `,`
		}
	}

	migrations = append(migrations, insertQuery)
	return migrations
}

func getPacketSequence(tx sdk.TxResponse) string {
	for _, log := range tx.Logs {
		for _, event := range log.Events {
			if event.Type == "send_packet" || event.Type == "recv_packet" {
				for _, attribute := range event.Attributes {
					if attribute.Key == "packet_sequence" {
						return attribute.Value
					}
				}
			}
		}
	}

	return ""
}
