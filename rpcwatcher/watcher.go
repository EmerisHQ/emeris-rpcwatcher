package rpcwatcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/emerishq/emeris-rpcwatcher/rpcwatcher/database"
	"github.com/emerishq/emeris-utils/store"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	liquiditytypes "github.com/gravity-devs/liquidity/x/liquidity/types"
	tmjson "github.com/tendermint/tendermint/libs/json"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/rpc/jsonrpc/client"
	"github.com/tendermint/tendermint/types"
)

const ackSuccess = "AQ==" // Packet ack value is true when ibc is success and contains error message in all other cases
const nonZeroCodeErrFmt = "non-zero code on chain %s: %s"

const (
	EventsTx                = "tm.event='Tx'"
	EventsBlock             = "tm.event='NewBlock'"
	defaultWSClientReadWait = 30 * time.Second
	defaultWatchdogTimeout  = 20 * time.Second
	defaultReconnectionTime = 15 * time.Second
	defaultResubscribeSleep = 500 * time.Millisecond
	defaultTimeGap          = 750 * time.Millisecond
)

var (
	EventsToSubTo = []string{EventsTx, EventsBlock}

	StandardMappings = map[string][]DataHandler{
		EventsTx: {
			HandleMessage,
		},
		EventsBlock: {
			HandleNewBlock,
		},
	}
	CosmosHubMappings = map[string][]DataHandler{
		EventsTx: {
			HandleMessage,
		},
		EventsBlock: {
			HandleNewBlock,
			HandleCosmosHubBlock,
		},
	}
)

type DataHandler func(watcher *Watcher, event coretypes.ResultEvent)

type WsResponse struct {
	Event coretypes.ResultEvent `json:"result"`
}

type Events map[string][]string

type VerifyTraceResponse struct {
	VerifyTrace struct {
		IbcDenom  string `json:"ibc_denom"`
		BaseDenom string `json:"base_denom"`
		Verified  bool   `json:"verified"`
		Path      string `json:"path"`
		Trace     []struct {
			Channel          string `json:"channel"`
			Port             string `json:"port"`
			ChainName        string `json:"chain_name"`
			CounterpartyName string `json:"counterparty_name"`
		} `json:"trace"`
	} `json:"verify_trace"`
}

type Ack struct {
	Result string `json:"result"`
}

type Watcher struct {
	Name         string
	DataChannel  chan coretypes.ResultEvent
	ErrorChannel chan error

	eventTypeMappings map[string][]DataHandler
	apiUrl            string
	client            *client.WSClient
	d                 *database.Instance
	l                 *zap.SugaredLogger
	store             *store.Store
	runContext        context.Context
	endpoint          string
	grpcEndpoint      string
	subs              []string
	stopReadChannel   chan struct{}
	stopErrorChannel  chan struct{}
	watchdog          *watchdog
}

func NewWatcher(
	endpoint, chainName string,
	logger *zap.SugaredLogger,
	apiUrl, grpcEndpoint string,
	db *database.Instance,
	s *store.Store,
	subscriptions []string,
	eventTypeMappings map[string][]DataHandler,
) (*Watcher, error) {
	if len(eventTypeMappings) == 0 {
		return nil, fmt.Errorf("event type mappings cannot be empty")
	}

	for _, eventKind := range subscriptions {
		handlers, ok := eventTypeMappings[eventKind]
		if !ok || len(handlers) == 0 {
			return nil, fmt.Errorf("event %s found in subscriptions but no handler defined for it", eventKind)
		}
	}

	ws, err := client.NewWS(
		endpoint,
		"/websocket",
		client.ReadWait(defaultWSClientReadWait),
	)

	if err != nil {
		return nil, err
	}

	ws.SetLogger(zapLogger{
		z:         logger,
		chainName: chainName,
	})

	if err := ws.OnStart(); err != nil {
		return nil, err
	}

	wd := newWatchdog(defaultWatchdogTimeout)

	w := &Watcher{
		apiUrl:            apiUrl,
		d:                 db,
		client:            ws,
		l:                 logger,
		store:             s,
		Name:              chainName,
		endpoint:          endpoint,
		grpcEndpoint:      grpcEndpoint,
		subs:              subscriptions,
		eventTypeMappings: eventTypeMappings,
		stopReadChannel:   make(chan struct{}),
		DataChannel:       make(chan coretypes.ResultEvent),
		stopErrorChannel:  make(chan struct{}),
		ErrorChannel:      make(chan error),
		watchdog:          wd,
	}

	w.l.Debugw("creating rpcwatcher with config", "apiurl", apiUrl)

	for _, sub := range subscriptions {
		if err := w.client.Subscribe(context.Background(), sub); err != nil {
			return nil, fmt.Errorf("failed to subscribe, %w", err)
		}
	}

	wd.Start()

	go w.readChannel()

	go w.checkError()
	return w, nil
}

func Start(watcher *Watcher, ctx context.Context) {
	watcher.runContext = ctx
	go watcher.startChain(ctx)
}

func (w *Watcher) readChannel() {
	/*
		This thing uses nested selects because when we read from tendermint data channel, we should check first if
		the cancellation function has been called, and if yes we should return.

		Only after having done such check we can process the tendermint data.
	*/
	for {
		select {
		case <-w.stopReadChannel:
			return
		case <-w.watchdog.timeout:
			w.ErrorChannel <- fmt.Errorf("watchdog ticked, reconnect to websocket")
			return
		default:
			select {
			case data := <-w.client.ResponsesCh:
				if data.Error != nil {
					go func() {
						w.l.Debugw("writing error to error channel", "error", data.Error)
						w.ErrorChannel <- data.Error
					}()

					// if we get any kind of error from tendermint, exit: the reconnection routine will take care of
					// getting us up to speed again
					return
				}

				e := coretypes.ResultEvent{}
				if err := tmjson.Unmarshal(data.Result, &e); err != nil {
					w.l.Errorw("cannot unmarshal data into resultevent", "error", err, "chain", w.Name)
					continue
				}

				go func() {
					w.DataChannel <- e
				}()
			case <-time.After(defaultReconnectionTime):
				w.ErrorChannel <- fmt.Errorf("tendermint websocket hang, triggering reconnection")
				return
			}
		}
	}
}

func (w *Watcher) checkError() {
	for {
		select {
		case <-w.stopErrorChannel:
			return
		default:
			select { //nolint Intentional channel construct
			case err := <-w.ErrorChannel:
				if err != nil {
					storeErr := w.store.SetWithExpiry(w.Name, "false", 0)
					if storeErr != nil {
						w.l.Errorw("unable to set chain name to false", "store error", storeErr,
							"error", err)
					}
					w.l.Errorw("detected error", "chain_name", w.Name, "error", err)
					resubscribe(w)
					return
				}
			}
		}
	}
}

func resubscribe(w *Watcher) {
	count := 0
	for {
		err := w.store.SetWithExpiry(w.Name, "resubscribing", 0)
		if err != nil {
			w.l.Errorw("unable to set chain name with status resubscribing", "error", err)
		}

		time.Sleep(defaultResubscribeSleep)
		count++
		w.l.Debugw("this is count", "count", count)

		ww, err := NewWatcher(w.endpoint, w.Name, w.l, w.apiUrl, w.grpcEndpoint, w.d, w.store, w.subs, w.eventTypeMappings)
		if err != nil {
			w.l.Errorw("cannot resubscribe to chain", "name", w.Name, "endpoint", w.endpoint, "error", err)
			continue
		}

		ww.runContext = w.runContext
		w = ww

		Start(w, w.runContext)
		err = w.store.SetWithExpiry(w.Name, "true", 0)
		if err != nil {
			w.l.Errorw("unable to set chain name as true", "error", err)
		}

		w.l.Infow("successfully reconnected", "name", w.Name, "endpoint", w.endpoint)
		return
	}
}

func (w *Watcher) startChain(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			w.stopReadChannel <- struct{}{}
			w.stopErrorChannel <- struct{}{}
			w.l.Infof("watcher %s has been canceled", w.Name)
			return
		default:
			select { //nolint intentional channel construct
			case data := <-w.DataChannel:
				if data.Query == "" {
					continue
				}

				handlers, ok := w.eventTypeMappings[data.Query]
				if !ok {
					w.l.Warnw("got event subscribed that didn't have a event mapping associated", "chain", w.Name, "eventName", data.Query)
					continue
				}

				for _, handler := range handlers {
					handler(w, data)
				}
			}
		}

	}
}

func HandleMessage(w *Watcher, data coretypes.ResultEvent) {
	txHashSlice, exists := data.Events["tx.hash"]
	_, createPoolEventPresent := data.Events["create_pool.pool_name"]
	_, IBCSenderEventPresent := data.Events["ibc_transfer.sender"]
	_, IBCAckEventPresent := data.Events["fungible_token_packet.acknowledgement"]
	_, IBCReceivePacketEventPresent := data.Events["recv_packet.packet_sequence"]
	_, IBCTimeoutEventPresent := data.Events["timeout.refund_receiver"]
	_, SwapTransactionEventPresent := data.Events["swap_within_batch.pool_id"]

	if len(txHashSlice) == 0 {
		return
	}

	txHash := txHashSlice[0]
	chainName := w.Name
	eventTx := data.Data.(types.EventDataTx)
	height := eventTx.Height
	key := store.GetKey(chainName, txHash)

	w.l.Debugw("got message to handle", "chain name", chainName, "key", key, "is create lp", createPoolEventPresent, "is ibc", IBCSenderEventPresent, "is ibc recv", IBCReceivePacketEventPresent,
		"is ibc ack", IBCAckEventPresent, "is ibc timeout", IBCTimeoutEventPresent)

	w.l.Debugw("is simple ibc transfer"+
		"", "is it", exists && !createPoolEventPresent && !IBCSenderEventPresent && !IBCReceivePacketEventPresent && w.store.Exists(key))

	if eventTx.Result.Code != 0 {
		logStr := fmt.Sprintf(nonZeroCodeErrFmt, chainName, eventTx.Result.Log)

		w.l.Debugw("transaction error", "chainName", chainName, "txHash", txHash, "log", eventTx.Result.Log)

		if err := w.store.SetFailedWithErr(key, logStr, height); err != nil {
			w.l.Errorw("cannot set failed with err", "chain name", chainName, "error", err,
				"txHash", txHash, "code", eventTx.Result.Code)
		}

		return
	}
	// Handle case where a simple non-IBC transfer is being used.
	if exists && !createPoolEventPresent && !IBCSenderEventPresent && !IBCReceivePacketEventPresent &&
		!IBCAckEventPresent && !IBCTimeoutEventPresent && !SwapTransactionEventPresent && w.store.Exists(key) {
		if err := w.store.SetComplete(key, height); err != nil {
			w.l.Errorw("cannot set complete", "chain name", chainName, "error", err)
		}
		return
	}

	// Handle case where an LP is being created on the Cosmos Hub
	if createPoolEventPresent && chainName == "cosmos-hub" {
		w.l.Debugw("is create lp", "is it", createPoolEventPresent)
		HandleCosmosHubLPCreated(w, data, chainName, key, height)
		return
	}

	if SwapTransactionEventPresent && w.Name == "cosmos-hub" {
		HandleSwapTransaction(w, data, chainName, key, height)
		return
	}

	// Handle case where an IBC transfer is sent from the origin chain.
	if IBCSenderEventPresent {
		HandleIBCSenderEvent(w, data, chainName, txHash, key, height)
		return
	}

	// Handle case where IBC transfer is received by the receiving chain.
	if IBCReceivePacketEventPresent {
		HandleIBCReceivePacket(w, data, chainName, txHash, height)
		return
	}

	if IBCTimeoutEventPresent {
		HandleIBCTimeoutPacket(w, data, chainName, txHash, height)
		return
	}

	if IBCAckEventPresent {
		HandleIBCAckPacket(w, data, chainName, txHash, height)
	}

}

func HandleCosmosHubBlock(w *Watcher, data coretypes.ResultEvent) {
	w.l.Debugw("called HandleCosmosHubBlock")
	realData, ok := data.Data.(types.EventDataNewBlock)
	if !ok {
		panic("rpc returned data which is not of expected type")
	}

	time.Sleep(defaultTimeGap) // to handle the time gap between block production and event broadcast
	newHeight := realData.Block.Header.Height

	u := w.endpoint

	ru, err := url.Parse(u)
	if err != nil {
		w.l.Errorw("cannot parse url", "url_string", u, "error", err)
		return
	}

	vals := url.Values{}
	vals.Set("height", strconv.FormatInt(newHeight, 10))

	w.l.Debugw("asking for block", "height", newHeight)

	ru.Path = "block_results"
	ru.RawQuery = vals.Encode()

	res := bytes.Buffer{}

	resp, err := http.Get(ru.String())
	if err != nil {
		w.l.Errorw("cannot query node for block data", "error", err, "height", newHeight)
		return
	}

	if resp.StatusCode != http.StatusOK {
		w.l.Errorw("endpoint returned non-200 code", "code", resp.StatusCode, "height", newHeight)
		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	read, err := res.ReadFrom(resp.Body)
	if err != nil {
		w.l.Errorw("cannot read block data resp body into buffer", "height", newHeight, "error", err)
		return
	}

	if read == 0 {
		w.l.Errorw("read zero bytes from response body", "height", newHeight)
	}

	bs := store.NewBlocks(w.store)
	err = bs.Add(res.Bytes(), newHeight)
	if err != nil {
		w.l.Errorw("cannot set block to cache", "error", err, "height", newHeight)
		return
	}

	// creating a grpc ClientConn to perform RPCs
	grpcConn, err := grpc.Dial(
		w.grpcEndpoint,
		grpc.WithInsecure(),
	)
	if err != nil {
		w.l.Errorw("cannot create gRPC client", "error", err, "chain_name", w.Name, "address", w.grpcEndpoint)
		return
	}

	defer func() {
		if err := grpcConn.Close(); err != nil {
			w.l.Errorw("cannot close gRPC client", "error", err, "chain_name", w.Name)
		}
	}()

	liquidityQuery := liquiditytypes.NewQueryClient(grpcConn)
	poolsRes, err := liquidityQuery.LiquidityPools(context.Background(), &liquiditytypes.QueryLiquidityPoolsRequest{})
	if err != nil {
		w.l.Errorw("cannot get liquidity pools in blocks", "error", err, "height", newHeight)
	}

	bz, err := w.store.Cdc.MarshalJSON(poolsRes)
	if err != nil {
		w.l.Errorw("cannot marshal liquidity pools", "error", err, "height", newHeight)
	}

	// caching pools info
	err = w.store.SetWithExpiry("pools", string(bz), 0)
	if err != nil {
		w.l.Errorw("cannot set liquidity pools", "error", err, "height", newHeight)
	}

	paramsRes, err := liquidityQuery.Params(context.Background(), &liquiditytypes.QueryParamsRequest{})
	if err != nil {
		w.l.Errorw("cannot get liquidity params", "error", err, "height", newHeight)
	}

	bz, err = w.store.Cdc.MarshalJSON(paramsRes)
	if err != nil {
		w.l.Errorw("cannot marshal liquidity params", "error", err, "height", newHeight)
	}

	// caching liquidity params
	err = w.store.SetWithExpiry("params", string(bz), 0)
	if err != nil {
		w.l.Errorw("cannot set liquidity params", "error", err, "height", newHeight)
	}

	supplyQuery := banktypes.NewQueryClient(grpcConn)
	supplyRes, err := supplyQuery.TotalSupply(context.Background(), &banktypes.QueryTotalSupplyRequest{})
	if err != nil {
		w.l.Errorw("cannot get total supply", "error", err, "height", newHeight)
	}

	bz, err = w.store.Cdc.MarshalJSON(supplyRes)
	if err != nil {
		w.l.Errorw("cannot marshal total supply", "error", err, "height", newHeight)
	}

	// caching total supply
	err = w.store.SetWithExpiry("supply", string(bz), 0)
	if err != nil {
		w.l.Errorw("cannot set total supply", "error", err, "height", newHeight)
	}
}

func HandleNewBlock(w *Watcher, data coretypes.ResultEvent) {
	w.watchdog.Ping()
	w.l.Debugw("performed watchdog ping", "chain_name", w.Name)
	w.l.Debugw("new block", "chain_name", w.Name)

	realData, ok := data.Data.(types.EventDataNewBlock)
	if !ok {
		panic("rpc returned block data which is not of expected type")
	}

	if realData.Block == nil {
		w.l.Warnw("weird block received on rpc, it was empty while it shouldn't", "chain_name", w.Name)
	}

	b := store.NewBlocks(w.store)

	if err := b.SetLastBlockTime(realData.Block.Time, realData.Block.Height); err != nil {
		w.l.Errorw("cannot write last block time to store", "chain_name", w.Name, "error", err)
		return
	}
}

func HandleCosmosHubLPCreated(w *Watcher, data coretypes.ResultEvent, chainName, key string, height int64) {
	defer func() {
		if err := w.store.SetComplete(key, height); err != nil {
			w.l.Errorw("cannot set complete", "chain name", chainName, "error", err)
		}
	}()

	chain, err := w.d.Chain(chainName)
	if err != nil {
		w.l.Errorw("can't find chain cosmos-hub", "error", err)
		return
	}

	poolCoinDenom, ok := data.Events["create_pool.pool_coin_denom"]
	if !ok {
		w.l.Errorw("no field create_pool.pool_coin_denom in Events", "error", err)
		return
	}

	dd, err := formatDenom(w, data)
	if err != nil {
		w.l.Errorw("failed to format denom", "error", err)
		return
	}

	found := false
	for _, token := range chain.Denoms {
		if token.Name == poolCoinDenom[0] {
			token = dd
			found = true
		}
	}

	if !found {
		chain.Denoms = append(chain.Denoms, dd)
	}

	err = w.d.UpdateDenoms(chain)
	if err != nil {
		w.l.Errorw("failed to update chain", "error", err)
		return
	}
}

func HandleSwapTransaction(w *Watcher, data coretypes.ResultEvent, chainName, key string, height int64) {
	poolId, ok := data.Events["swap_within_batch.pool_id"]
	if !ok {
		w.l.Errorw("pool_id not found")
		return
	}

	offerCoinFee, ok := data.Events["swap_within_batch.offer_coin_fee_amount"]
	if !ok {
		w.l.Errorw("offer_coin_fee_amount not found")
		return
	}

	offerCoinDenom, ok := data.Events["swap_within_batch.offer_coin_denom"]
	if !ok {
		w.l.Errorw("offer_coin_fee_denom not found")
	}

	err := w.store.SetPoolSwapFees(poolId[0], offerCoinFee[0], offerCoinDenom[0])
	if err != nil {
		w.l.Errorw("unable to store swap fees", "error", err)
	}

	if err := w.store.SetComplete(key, height); err != nil {
		w.l.Errorw("cannot set complete", "chain name", chainName, "error", err)
	}
}

func HandleIBCSenderEvent(w *Watcher, data coretypes.ResultEvent, chainName, txHash, key string, height int64) {
	sendPacketSourcePort, ok := data.Events["send_packet.packet_src_port"]
	if !ok {
		w.l.Errorf("send_packet.packet_src_port not found")
		return
	}

	if sendPacketSourcePort[0] != "transfer" {
		w.l.Errorf("port is not 'transfer', ignoring")
		return
	}

	sendPacketSourceChannel, ok := data.Events["send_packet.packet_src_channel"]
	if !ok {
		w.l.Errorf("send_packet.packet_src_channel not found")
		return
	}

	sendPacketSequence, ok := data.Events["send_packet.packet_sequence"]
	if !ok {
		w.l.Errorf("send_packet.packet_sequence not found")
		return
	}

	c, err := w.d.GetCounterParty(chainName, sendPacketSourceChannel[0])
	if err != nil {
		w.l.Errorw("unable to fetch counterparty chain from db", "error", err)
		return
	}

	if err := w.store.SetInTransit(key, c[0].Counterparty, sendPacketSourceChannel[0], sendPacketSequence[0],
		txHash, chainName, height); err != nil {
		w.l.Errorw("unable to set status as in transit for key", "key", key, "error", err)
	}
}

func HandleIBCReceivePacket(w *Watcher, data coretypes.ResultEvent, chainName, txHash string, height int64) {
	w.l.Debugw("called HandleIBCReceivePacket")
	recvPacketSourcePort, ok := data.Events["recv_packet.packet_src_port"]
	if !ok {
		w.l.Errorf("recv_packet.packet_src_port not found")
		return
	}

	if recvPacketSourcePort[0] != "transfer" {
		w.l.Errorf("port is not 'transfer', ignoring")
		return
	}

	recvPacketSourceChannel, ok := data.Events["recv_packet.packet_src_channel"]
	if !ok {
		w.l.Errorf("recv_packet.packet_src_channel not found")
		return
	}

	recvPacketSequence, ok := data.Events["recv_packet.packet_sequence"]
	if !ok {
		w.l.Errorf("recv_packet.packet_sequence not found")
		return
	}

	packetAck, ok := data.Events["write_acknowledgement.packet_ack"]
	if !ok {
		w.l.Errorf("packet ack not found")
		return
	}

	key := store.GetIBCKey(chainName, recvPacketSourceChannel[0], recvPacketSequence[0])
	if !w.store.Exists(key) {
		w.l.Debugw("bypassing key, event not sourced from us", "chain_name", w.Name, "key", key, "event", "ibc_receive")
		return
	}

	var ack Ack
	if err := json.Unmarshal([]byte(packetAck[0]), &ack); err != nil {
		w.l.Errorw("unable to unmarshal packetAck", "err", err)
		return
	}

	if ack.Result != ackSuccess {
		if err := w.store.SetIbcFailed(key, txHash, chainName, height); err != nil {
			w.l.Errorw("unable to set status as failed for key", "key", key, "error", err)
		}
		return
	}

	if err := w.store.SetIbcReceived(key, txHash, chainName, height); err != nil {
		w.l.Errorw("unable to set status as ibc received for key", "key", key, "error", err)
	}
}

func HandleIBCTimeoutPacket(w *Watcher, data coretypes.ResultEvent, chainName, txHash string, height int64) {
	timeoutPacketSourceChannel, ok := data.Events["timeout_packet.packet_src_channel"]
	if !ok {
		w.l.Errorf("timeout_packet.packet_src_channel not found")
		return
	}

	timeoutPacketSequence, ok := data.Events["timeout_packet.packet_sequence"]
	if !ok {
		w.l.Errorf("timeout_packet.packet_sequence not found")
		return
	}

	c, err := w.d.GetCounterParty(chainName, timeoutPacketSourceChannel[0])
	if err != nil {
		w.l.Errorw("unable to fetch counterparty chain from db", "error", err)
		return
	}

	key := store.GetIBCKey(c[0].Counterparty, timeoutPacketSourceChannel[0], timeoutPacketSequence[0])
	if !w.store.Exists(key) {
		w.l.Debugw("bypassing key, event not sourced from us", "chain_name", w.Name, "key", key, "event", "timeout")
		return
	}

	if err := w.store.SetIbcTimeoutUnlock(key, txHash, chainName, height); err != nil {
		w.l.Errorw("unable to set status as ibc timeout unlock for key", "key", key, "error", err)
	}
}

func HandleIBCAckPacket(w *Watcher, data coretypes.ResultEvent, chainName, txHash string, height int64) {
	w.l.Debugw("called HandleIBCAckPacket")
	ackPacketSourceChannel, ok := data.Events["acknowledge_packet.packet_src_channel"]
	if !ok {
		w.l.Errorf("acknowledge_packet.packet_src_channel not found")
		return
	}

	ackPacketSequence, ok := data.Events["acknowledge_packet.packet_sequence"]
	if !ok {
		w.l.Errorf("acknowledge_packet.packet_sequence not found")
		return
	}

	c, err := w.d.GetCounterParty(chainName, ackPacketSourceChannel[0])
	if err != nil {
		w.l.Errorw("unable to fetch counterparty chain from db", "error", err)
		return
	}

	key := store.GetIBCKey(c[0].Counterparty, ackPacketSourceChannel[0], ackPacketSequence[0])
	_, ok = data.Events["fungible_token_packet.error"]
	if ok {
		if !w.store.Exists(key) {
			w.l.Debugw("bypassing key, event not sourced from us", "chain_name", w.Name, "key", key, "event", "ibc_ack")
			return
		}

		if err := w.store.SetIbcAckUnlock(key, txHash, chainName, height); err != nil {
			w.l.Errorw("unable to set status as ibc ack unlock for key", "key", key, "error", err)
		}
		return
	}
}
