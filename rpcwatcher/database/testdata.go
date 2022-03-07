package database

const (
	TestChainName  = "cosmos-hub"
	CreateDB       = `CREATE DATABASE IF NOT EXISTS cns`
	CreateCNSTable = `CREATE TABLE IF NOT EXISTS cns.chains (
		id serial unique primary key,
		enabled boolean default false,
		chain_name string not null,
		valid_block_thresh string not null,
		logo string not null,
		display_name string not null,
		primary_channel jsonb not null,
		denoms jsonb not null,
		demeris_addresses text[] not null,
		genesis_hash string not null,
		node_info jsonb not null,
		derivation_path string not null,
		unique(chain_name)
		)`
)

var (
	TestDBMigrations = []string{
		CreateDB,
		CreateCNSTable,
		`
	INSERT INTO cns.chains 
		(
		id, enabled, chain_name, valid_block_thresh, logo, display_name, primary_channel, denoms, demeris_addresses, 
		genesis_hash, node_info, derivation_path
		) 
		VALUES (
			1, true, '` + TestChainName + `', '10s', 'logo url', 'Cosmos Hub', 
			'{"cosmos-hub": "channel-0", "akash": "channel-1"}',
			'[
				{"display_name":"USD","name":"uusd","verified":true,"fee_token":true,"fetch_price":true,"fee_levels":{"low":1,"average":22,"high":42},"precision":6},
				{"display_name":"ATOM","name":"uatom","verified":true,"fetch_price":true,"fee_token":true,"fee_levels":{"low":1,"average":22,"high":42},"precision":6}
			]', 
			ARRAY['feeaddress'], 'genesis_hash', 
			'{"endpoint":"endpoint","chain_id":"chainid","bech32_config":{"main_prefix":"main_prefix","prefix_account":"prefix_account","prefix_validator":"prefix_validator",
			"prefix_consensus":"prefix_consensus","prefix_public":"prefix_public","prefix_operator":"prefix_operator"}}', 
			'm/44''/118''/0''/0/0')
	`,
	}
)
