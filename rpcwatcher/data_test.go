package rpcwatcher

var testDBMigrations = []string{
	`CREATE DATABASE IF NOT EXISTS cns`,
	`CREATE TABLE IF NOT EXISTS cns.chains (
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
	)`,
	`
	INSERT INTO cns.chains 
		(
		id, enabled, chain_name, valid_block_thresh, logo, display_name, primary_channel, denoms, demeris_addresses, 
		genesis_hash, node_info, derivation_path
		) 
		VALUES (
			1, true, 'cosmos-hub', '10s', 'logo url', 'Cosmos Hub', 
			'{"cn1": "cn1", "cn2": "cn2"}',
			'[
				{"display_name":"STAKE","name":"stake","verified":true,"fee_token":true,"fetch_price":true,"fee_levels":{"low":1,"average":22,"high":42},"precision":6},
				{"display_name":"ATOM","name":"uatom","verified":true,"fetch_price":true,"fee_token":true,"fee_levels":{"low":1,"average":22,"high":42},"precision":6}
			]', 
			ARRAY['feeaddress'], 'genesis_hash', 
			'{"endpoint":"endpoint","chain_id":"chainid","bech32_config":{"main_prefix":"main_prefix","prefix_account":"prefix_account","prefix_validator":"prefix_validator",
			"prefix_consensus":"prefix_consensus","prefix_public":"prefix_public","prefix_operator":"prefix_operator"}}', 
			'm/44''/118''/0''/0/0')
	`,
	`
	INSERT INTO cns.chains 
		(
		id, enabled, chain_name, valid_block_thresh, logo, display_name, primary_channel, denoms, demeris_addresses, 
		genesis_hash, node_info, derivation_path
		) 
		VALUES (
			2, true, 'akash', '10s', 'logo url', 'Akash Network', 
			'{"cn3": "cn3", "cn4": "cn4"}',
			'[
				{"display_name":"AKT","name":"uakt","verified":true,"fetch_price":true,"fee_token":true,"fee_levels":{"low":1,"average":22,"high":42},"precision":6}
			]', 
			ARRAY['feeaddress2'], 'genesis_hash_2', 
			'{"endpoint":"endpoint2","chain_id":"chainid2","bech32_config":{"main_prefix":"main_prefix","prefix_account":"prefix_account","prefix_validator":"prefix_validator",
			"prefix_consensus":"prefix_consensus","prefix_public":"prefix_public","prefix_operator":"prefix_operator"}}', 
			'm/44''/118''/0''/0/0')
	`,
}

const defaultEventData = `{
	"query":"tm.event='Tx'",
	"data":{},
	"events":`

var nonIBCTransferEvent = defaultEventData + `{
		"delegate.amount":["20000000"],
		"delegate.validator":["cosmosvaloper1ep8nr7d383ng7pndvl2u9j6c9tdkwyr6dvtn2n"],
		"message.action":["delegate"],
		"message.module":["staking"],
		"message.sender":["cosmos1vaa40n5naka7mav3za6kx40jckx6aa4nqvvx8a","cosmos1jv65s3grqf6v6jl3dp4t6c9t9rk99cd82yfms9"],
		"tm.event":["Tx"],
		"transfer.amount":["500uatom","982uatom"],
		"transfer.recipient":["cosmos17xpfvakm2amg962yls6f84z3kell8c5lazw8j8"],
		"transfer.sender":["cosmos1vaa40n5naka7mav3za6kx40jckx6aa4nqvvx8a","cosmos1jv65s3grqf6v6jl3dp4t6c9t9rk99cd82yfms9"],
		"tx.hash":["D88B758F52DD059958B55B5874105BDF421F0685BA7EB1ACBC7D04337D6466E0"],
		"tx.height":["3502864"]
}}`

var ibcTransferEvent = defaultEventData + `{
	"message.sender": ["cosmos16ma9usaqqgz0mtfkfhpnf767cqkz5p7htlt8xy"],
	"send_packet.packet_timeout_height": ["0-1267"],
	"send_packet.packet_dst_port": ["transfer"],
	"send_packet.packet_dst_channel": ["channel-0"],
	"send_packet.packet_timeout_timestamp": ["1621490047681380000"],
	"send_packet.packet_src_port": ["transfer"],
	"ibc_transfer.sender": ["cosmos16ma9usaqqgz0mtfkfhpnf767cqkz5p7htlt8xy"],
	"ibc_transfer.receiver": ["cosmos1v4a9gud6ycj7pd2fl7g563y3rxgyn6yjg0w7g2"],
	"message.action": ["transfer"],
	"transfer.recipient": ["cosmos1a53udazy8ayufvy0s434pfwjcedzqv34kvz9tw"],
	"transfer.amount": ["100token"],
	"send_packet.packet_data": [
		"{\"amount\":\"100\",\"denom\":\"token\",\"receiver\":\"cosmos1v4a9gud6ycj7pd2fl7g563y3rxgyn6yjg0w7g2\",
		\"sender\":\"cosmos16ma9usaqqgz0mtfkfhpnf767cqkz5p7htlt8xy\"}"
	],
	"tm.event": ["Tx"],
	"tx.hash": ["60895FD5A77581E3F5969CB3B2E34127A9267CFD2CC9EFDEA36384D6F3E35B79"],
	"send_packet.packet_sequence": ["1"],
	"send_packet.packet_channel_ordering": ["ORDER_UNORDERED"],
	"message.module": ["ibc_channel", "transfer"],
	"transfer.sender": ["cosmos16ma9usaqqgz0mtfkfhpnf767cqkz5p7htlt8xy"],
	"send_packet.packet_src_channel": ["channel-0"],
	"send_packet.packet_connection": ["connection-0"],
	"tx.height": ["76376"]
}}`

var ibcAckTxEvent = defaultEventData + `{
	"fungible_token_packet.acknowledgement": ["{0xc007f5c450}"],
        "fungible_token_packet.success": ["\u0001"],
        "message.action": ["update_client","acknowledge_packet"],
        "update_client.client_type": ["07-tendermint"],
        "acknowledge_packet.packet_dst_channel": ["channel-0"],
        "acknowledge_packet.packet_connection": ["connection-0"],
        "fungible_token_packet.receiver": ["cosmos1v4a9gud6ycj7pd2fl7g563y3rxgyn6yjg0w7g2"],
        "fungible_token_packet.amount": ["100"],
        "update_client.client_id": ["07-tendermint-0"],
        "update_client.consensus_height": ["0-370"],
        "acknowledge_packet.packet_src_port": ["transfer"],
        "acknowledge_packet.packet_src_channel": ["channel-0"],
        "fungible_token_packet.denom": ["token"],
        "tx.hash": ["EB782364E4F6412482555616B87DF077B6846B3CCC9206AF5136120D70D24B03"],
        "fungible_token_packet.module": ["transfer"],
        "update_client.header": ["0a262f6962632e6c69676874636c69656e74732e74656e6465726d696e742e76312e48656164657212cf060ac9040a8c03
		0a02080b1205746573743218f202220c089bef97850610889fdcf5022a480a20cb1cc81e5fec2d7a80475de0de4ea5783c714f478e43b6db7dece7cf691cefdc12240
		80112200e4f69d7183a4ada7167664699825f3a8dd4f4b1e17acd93d88fdefc02eb262a3220fddc19b047a27e86c80247a5d6469b89324a933a9027db8d9aabb
		a865ba0e05b3a20e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b85542200712d54773a5c7615f6321e1cf5
		d34d4602d5f4d0483e40a06fec75a082c6bab4a200712d54773a5c7615f6321e1cf5d34d4602d5f4d0483e40a06fec75a082c6bab5220048091bc
		7ddc283f77bfbf91d73c44da58c3df8a9cbc867405d8b7f3daada22f5a20c67b40e16528d58b0e977104d2a83357894a494fceede262994dca4d6
		d7e181e6220d6b2128e54a77e660ae11cd1ade597a14a9429324f5d99be4e2e6d81644027b26a20e3b0c44298fc1c149afbf4c8996fb92427ae41
		e4649b934ca495991b7852b8557214572090b3d7d200996a6c425028b99b232054934612b70108f2021a480a2037fa100fd781da6977470083d103d8
		09129a266307483e35a1cb76eed3fbeb9d122408011220041790150a41f0694896a9878c431831484d645ff11a1a25552f870203e79965226808021214572090
		b3d7d200996a6c425028b99b23205493461a0c089cef9785061080bd93d9032240a2292f246e099a9385f74fa6074e6d4c9be0e3688190e85a9d6b4feb244db8
		9c247897d2ac39822ffae108aa5f21494c2ef362b0500fa6281a381a81a3bd0509127e0a3c0a14572090b3d7d200996a6c425028b99b232054934612220a206fbc5
		52311271853b2fb5e7b14c6bcb7ae2cbd0862694c52e1d4fe781886fcd81864123c0a14572090b3d7d200996a6c425028b99b232054934612220a206fbc552311
		271853b2fb5e7b14c6bcb7ae2cbd0862694c52e1d4fe781886fcd8186418641a03108b02227c0a3c0a14572090b3d7d200996a6c425028b99b23205493461222
		0a206fbc552311271853b2fb5e7b14c6bcb7ae2cbd0862694c52e1d4fe781886fcd81864123c0a14572090b3d7d200996a6c425028b99b232054934612220a206fbc
		552311271853b2fb5e7b14c6bcb7ae2cbd0862694c52e1d4fe781886fcd81864"],
        "message.module": ["ibc_client","ibc_channel"],
        "acknowledge_packet.packet_timeout_height": ["0-1267"],
        "acknowledge_packet.packet_timeout_timestamp": ["1621490047681380000"],
        "acknowledge_packet.packet_dst_port": ["transfer"],
        "acknowledge_packet.packet_channel_ordering": ["ORDER_UNORDERED"],
        "acknowledge_packet.packet_sequence": ["1"],
        "tm.event": ["Tx"],
        "tx.height": ["76383"]
}}`

var ibcReceivePacketEvent = defaultEventData + `{
	"update_client.consensus_height": ["0-76377"],
    "transfer.sender": ["cosmos1yl6hdjhmkf37639730gffanpzndzdpmhwlkfhr"],
    "message.sender": ["cosmos1yl6hdjhmkf37639730gffanpzndzdpmhwlkfhr"],
    "fungible_token_packet.module": ["transfer"],
    "write_acknowledgement.packet_sequence": ["1"],
    "write_acknowledgement.packet_src_channel": ["channel-0"],
    "write_acknowledgement.packet_dst_channel": ["channel-0"],
    "message.action": ["update_client", "recv_packet"],
    "tx.height": ["369"],
    "fungible_token_packet.success": ["false"],
    "fungible_token_packet.denom": ["token"],
    "fungible_token_packet.amount": ["100"],
    "transfer.amount": ["100ibc/B5CB286F69D48B2C4F6F8D8CF59011C40590DCF8A91617A5FBA9FF0A7B21307F"],
    "recv_packet.packet_connection": ["connection-0"],
    "denomination_trace.trace_hash": ["B5CB286F69D48B2C4F6F8D8CF59011C40590DCF8A91617A5FBA9FF0A7B21307F"],
    "recv_packet.packet_sequence": ["1"],
    "update_client.header": ["0a262f6962632e6c69676874636c69656e74732e74656e6465726d696e742e76312e48656164657212cf06
	0ac8040a8b030a02080b12047465737418d9d404220b0899ef97850610b0c798182a480a20e24f9e2e1265f77850fee30533f6e8ac8116
	aa198ccfb7f0fee21d6f8ef844141224080112205595952bf1f9818fbc8da8cfa9930449e9ffd10427a9bf371848feca002ed50d3220
	dabf945f2783c214e1e397b3775c998aab67ef896c64c70a70a199818db2f79b3a20e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b9
	34ca495991b7852b855422091519719cbcb07b33a0b468253973465fc61eac1e4948d2d8bf095fdae13bc284a2091519719cbcb07b33a0b468
	253973465fc61eac1e4948d2d8bf095fdae13bc285220048091bc7ddc283f77bfbf91d73c44da58c3df8a9cbc867405d8b7f3daada22f5a20b
	7dccb2fd77123f5c92a7b6a6d14b948d6b3d32e1e44236b12f45b31a03f18c26220049542f20abb2c8b10f03fc75a3a8a312f2040b6a5c8628
	63cc4ea4858184ee56a20e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855721427cefff1e12f5f040e57cb4fe
	4bc4bf8988690d712b70108d9d4041a480a20404267258fa8e0621629a4d7970a7515835be089e99d4624a63e2c6d50e3e8191224080112204
	e68da645253d99e2b50a11b89bc8a87543dcac02c618ef6730e5bdb9cfd7a5622670802121427cefff1e12f5f040e57cb4fe4bc4bf8988690d7
	1a0b089aef97850610c8eda8532240e24f7e6c520c4fc229bc5b241345a02ab2aeefd6e09719c4530c58438e22a7e965c814f361366bf5a0b9c
	c992bdb53688f885999c578bd2fe2ee76b532fbf305127e0a3c0a1427cefff1e12f5f040e57cb4fe4bc4bf8988690d712220a20df7e38431ba
	7663a642d575648f1aa46ed7eb029bfe0499c29a848c81c10926e1864123c0a1427cefff1e12f5f040e57cb4fe4bc4bf8988690d712220a20d
	f7e38431ba7663a642d575648f1aa46ed7eb029bfe0499c29a848c81c10926e186418641a0410fdd304227c0a3c0a1427cefff1e12f5f040e5
	7cb4fe4bc4bf8988690d712220a20df7e38431ba7663a642d575648f1aa46ed7eb029bfe0499c29a848c81c10926e1864123c0a1427cefff1e
	12f5f040e57cb4fe4bc4bf8988690d712220a20df7e38431ba7663a642d575648f1aa46ed7eb029bfe0499c29a848c81c10926e1864"],
    "recv_packet.packet_data": [
		"{\"amount\":\"100\",\"denom\":\"token\",\"receiver\":\"cosmos1v4a9gud6ycj7pd2fl7g563y3rxgyn6yjg0w7g2\",
		\"sender\":\"cosmos16ma9usaqqgz0mtfkfhpnf767cqkz5p7htlt8xy\"}"
	],
    "recv_packet.packet_channel_ordering": ["ORDER_UNORDERED"],
    "write_acknowledgement.packet_data": [
		"{\"amount\":\"100\",\"denom\":\"token\",\"receiver\":\"cosmos1v4a9gud6ycj7pd2fl7g563y3rxgyn6yjg0w7g2\",
		\"sender\":\"cosmos16ma9usaqqgz0mtfkfhpnf767cqkz5p7htlt8xy\"}"
	],
    "write_acknowledgement.packet_timeout_height": ["0-1267"],
    "update_client.client_type": ["07-tendermint"],
    "recv_packet.packet_src_port": ["transfer"],
    "recv_packet.packet_dst_channel": ["channel-0"],
    "denomination_trace.denom": ["ibc/B5CB286F69D48B2C4F6F8D8CF59011C40590DCF8A91617A5FBA9FF0A7B21307F"],
    "fungible_token_packet.receiver": ["cosmos1v4a9gud6ycj7pd2fl7g563y3rxgyn6yjg0w7g2"],
    "write_acknowledgement.packet_timeout_timestamp": ["1621490047681380000"],
    "write_acknowledgement.packet_dst_port": ["transfer"],
    "write_acknowledgement.packet_connection": ["connection-0"],
    "update_client.client_id": ["07-tendermint-0"],
    "recv_packet.packet_timeout_height": ["0-1267"],
    "recv_packet.packet_src_channel": ["channel-0"],
    "recv_packet.packet_dst_port": ["transfer"],
    "transfer.recipient": ["cosmos1v4a9gud6ycj7pd2fl7g563y3rxgyn6yjg0w7g2"],
    "tx.hash": ["A25BEE3715F405D4EF37712B8BA5ED2016655B37042BE0F974C3C850B94395EA"],
    "message.module": ["ibc_client", "ibc_channel",
    "write_acknowledgement.packet_src_port": ["transfer"],
    "write_acknowledgement.packet_ack": ["{\"result\":\"AQ==\"}"],
    "tm.event": ["Tx"],
    "recv_packet.packet_timeout_timestamp": ["1621490047681380000"]
}}`

var ibcTimeoutEvent = defaultEventData + `{
	"message.action":["timeout_packet"],
	"message.module":["ibc_channel"],
	"message.sender":["cosmos1akfrf46rjl52wqrfkm79dyr6vv32fcs6qj3ef8"],
	"timeout.module": ["transfer"],
	"timeout.refund_receiver": ["cosmos1zh3atlxnu9q0nvsz6yldjl3n76j7wha7f43kpd"],
	"timeout.refund_denom": ["uatom"],
	"timeout.refund_amount": ["100"],
	"timeout_packet.packet_timeout_height":["2-1226911"],
	"timeout_packet.packet_timeout_timestamp":["0],
	"timeout_packet.packet_sequence":["827"],
	"timeout_packet.packet_src_port":["transfer"],
	"timeout_packet.packet_src_channel":["channel-95"],
	"timeout_packet.packet_dst_port":["transfer"],
	"timeout_packet.packet_dst_channel":["channel-0"],
	"timeout_packet.packet_channel_ordering":["ORDER_UNORDERED"],
	"tm.event":["Tx"],
	"transfer.amount":["100uatom"],
	"transfer.recipient":["cosmos1zh3atlxnu9q0nvsz6yldjl3n76j7wha7f43kpd"],
	"transfer.sender":["cosmos1akfrf46rjl52wqrfkm79dyr6vv32fcs6qj3ef8"],
	"tx.hash":["E5798B3CF26BC5287BABCB8B35CC2E4526B98A2B16099C2AC7C48A3D30621EDF"],
	"tx.height":["6475075"]
}}`

var createPoolEvent = defaultEventData + `{
	"create_pool.deposit_coins": ["1000000000uatom,50000000000uusd"],
	"tx.height": ["52"],
	"message.action": ["create_pool"],
	"transfer.sender": ["cosmos1gerupkxgr4xq9a58pjrps0dzdelenkjyewu4ss"],
	"create_pool.pool_name": ["uatom/uusd/1"],
	"create_pool.reserve_account": ["cosmos1jmhkafh94jpgakr735r70t32sxq9wzkayzs9we"],
	"message.sender": [
		"cosmos1gerupkxgr4xq9a58pjrps0dzdelenkjyewu4ss",
		"cosmos1tx68a8k9yz54z06qfve9l2zxvgsz4ka3hr8962",
		"cosmos1gerupkxgr4xq9a58pjrps0dzdelenkjyewu4ss"
	],
	"create_pool.pool_id": ["1"],
	"create_pool.pool_type_id": ["1"],
	"create_pool.pool_coin_denom": ["pool96EF6EA6E5AC828ED87E8D07E7AE2A8180570ADD212117B2DA6F0B75D17A6295"],
	"transfer.recipient": [
		"cosmos1jmhkafh94jpgakr735r70t32sxq9wzkayzs9we",
		"cosmos1gerupkxgr4xq9a58pjrps0dzdelenkjyewu4ss",
		"cosmos1jv65s3grqf6v6jl3dp4t6c9t9rk99cd88lyufl"
	],
	"transfer.amount": [
	  "1000000000uatom,50000000000uusd",
	  "1000000pool96EF6EA6E5AC828ED87E8D07E7AE2A8180570ADD212117B2DA6F0B75D17A6295",
	  "100000000stake"
	],
	"tm.event": ["Tx"],
	"message.module": ["liquidity"],
	"tx.hash": [
	  "FE28734FEDC763A233AD14154C71B2391835E03F47B333703A32CAAA5265015C"
	]
}}`

var swapTransactionEvent = defaultEventData + `{
	"message.action":["swap_within_batch"],
	"message.module":["liquidity"],
	"message.sender":["cosmos16uu5kxmdpnq4e43u9tsyg22l5v7wlnn4j32zl8"],
	"swap_within_batch.pool_id":"5",
	"swap_within_batch.batch_index":"23941",
	"swap_within_batch.msg_index":"19708",
	"swap_within_batch.swap_type_id":"1",
	"swap_within_batch.offer_coin_denom":"ibc/2181AAB0218EAC24BC9F86BD1364FBBFA3E6E3FCC25E88E3E68C15DC6E752D86",
	"swap_within_batch.offer_coin_amount":"2000157755",
	"swap_within_batch.offer_coin_fee_amount":"3000236",
	"swap_within_batch.demand_coin_denom":"uatom",
	"swap_within_batch.order_price":"12.429270876237939802",
	"tm.event":["Tx"],
	"transfer.amount":["2003157991ibc/2181AAB0218EAC24BC9F86BD1364FBBFA3E6E3FCC25E88E3E68C15DC6E752D86"],
	"transfer.recipient":["cosmos1tx68a8k9yz54z06qfve9l2zxvgsz4ka3hr8962"],
	"transfer.sender":["cosmos16uu5kxmdpnq4e43u9tsyg22l5v7wlnn4j32zl8"],
	"tx.hash":["D2BA6EFCE89615AF81130A2C8B4E8A7E7D36864BEF11CE329ADD9AB80DFEE1EE"],
	"tx.height":["8407895"]
}}`

const resultCodeZero = `{"height":3502864,"tx":"","result":{"data":"CgoKCGRlbGVnYXRl","log":"[]","code":0, "gas_wanted":200000,"gas_used":149189,"events":[]}}`
const resultCodeNonZero = `{"height":3502865,"tx":"","result":{"data":"CgoKCGRlbGVnYXRz","log":"[]","code":19, "gas_wanted":200000,"gas_used":149189,"events":[]}}`
