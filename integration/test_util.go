package integration

import (
	"fmt"
	"reflect"

	"github.com/allinbits/emeris-rpcwatcher/rpcwatcher/database"
	"github.com/allinbits/emeris-utils/store"
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
	value += `}',
	'[`

	for index, v := range chain.denoms {
		value = value + `{"display_name":"` + v.displayDenom + `","name":"` + v.denom + `","verified":true,"fetch_price":true,"fee_token":true,"fee_levels":{"low":1,"average":22,"high":42},"precision":6}`
		if index != len(chain.denoms)-1 {
			value += `,`
		}
	}

	value += `]', 
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

func checkTxHashEntry(ticket store.Ticket, txHashEntry store.TxHashEntry) bool {
	for _, entry := range ticket.TxHashes {
		if reflect.DeepEqual(entry, txHashEntry) {
			return true
		}
	}
	return false
}

func checkFungibleTokenErr(tx sdk.TxResponse) bool {
	for _, log := range tx.Logs {
		for _, event := range log.Events {
			if event.Type == "fungible_token_packet" {
				for _, attribute := range event.Attributes {
					if attribute.Key == "error" {
						return true
					}
				}
			}
		}
	}

	return false
}
