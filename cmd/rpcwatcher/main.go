package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/r3labs/diff"
	"go.uber.org/zap"

	cnsmodels "github.com/allinbits/demeris-backend-models/cns"
	"github.com/allinbits/emeris-rpcwatcher/rpcwatcher"
	"github.com/allinbits/emeris-rpcwatcher/rpcwatcher/database"
	"github.com/allinbits/emeris-utils/logging"
	"github.com/allinbits/emeris-utils/store"

	_ "net/http/pprof"
)

var Version = "not specified"

type watcherInstance struct {
	watcher *rpcwatcher.Watcher
	cancel  context.CancelFunc
}

func main() {
	c, err := rpcwatcher.ReadConfig()
	if err != nil {
		panic(err)
	}

	l := logging.New(logging.LoggingConfig{
		Debug: c.Debug,
		JSON:  c.JSONLogs,
	})

	l.Infow("rpcwatcher", "version", Version)

	if c.Debug {
		go func() {
			l.Debugw("starting profiling server", "address", c.ProfilingServerURL)
			err := http.ListenAndServe(c.ProfilingServerURL, nil)
			if err != nil {
				l.Panicw("cannot run profiling server", "error", err)
			}
		}()
	}

	db, err := database.New(c.DatabaseConnectionURL)

	if err != nil {
		panic(err)
	}

	s, err := store.NewClient(c.RedisURL)
	if err != nil {
		l.Panicw("unable to start redis client", "error", err)
	}
	var chains []cnsmodels.Chain

	watchers := map[string]watcherInstance{}

	chains, err = db.Chains()

	if err != nil {
		panic(err)
	}

	chainsMap := mapChains(chains)

	for cn := range chainsMap {
		updatedChainsMap, watcher, cancel, shouldContinue := startNewWatcher(cn, chainsMap, c, db, s, l, false)
		chainsMap = updatedChainsMap
		if shouldContinue {
			continue
		}

		watchers[cn] = watcherInstance{
			watcher: watcher,
			cancel:  cancel,
		}
	}

	for range time.Tick(1 * time.Second) {

		ch, err := db.Chains()
		if err != nil {
			l.Errorw("cannot get chains from db", "error", err)
			continue
		}

		newChainsMap := mapChains(ch)

		chainsDiff, err := diff.Diff(chainsMap, newChainsMap)
		if err != nil {
			l.Errorw("cannot diff maps", "error", err)
			continue
		}

		if chainsDiff == nil {
			continue
		}

		l.Debugw("diff", "diff", chainsDiff)
		for _, d := range chainsDiff {
			switch d.Type {
			case diff.DELETE:
				name := d.Path[0]
				wi, ok := watchers[name]
				if !ok {
					// we probably deleted this already somehow
					continue
				}
				wi.cancel()

				delete(watchers, name)
				delete(chainsMap, name)
			case diff.CREATE:
				name := d.Path[0]

				_, watcher, cancel, shouldContinue := startNewWatcher(name, chainsMap, c, db, s, l, true)
				if shouldContinue {
					continue
				}

				watchers[name] = watcherInstance{
					watcher: watcher,
					cancel:  cancel,
				}

				chainsMap[name] = newChainsMap[name]
			}
		}
	}
}

func startNewWatcher(chainName string, chainsMap map[string]cnsmodels.Chain, config *rpcwatcher.Config, db *database.Instance, s *store.Store,
	l *zap.SugaredLogger, isNewChain bool) (map[string]cnsmodels.Chain, *rpcwatcher.Watcher, context.CancelFunc, bool) {
	eventMappings := rpcwatcher.StandardMappings

	if chainName == "cosmos-hub" { // special case, needs to observe new blocks too
		eventMappings = rpcwatcher.CosmosHubMappings
	}

	watcher, err := rpcwatcher.NewWatcher(endpoint(chainName), chainName, l, config.ApiURL, db, s, rpcwatcher.EventsToSubTo, eventMappings)

	if err != nil {
		if isNewChain {
			var dnsErr *net.DNSError
			if errors.As(err, &dnsErr) || strings.Contains(err.Error(), "connection refused") {
				l.Infow("chain not yet available", "name", chainName)
				return chainsMap, nil, nil, true
			}
		} else {
			delete(chainsMap, chainName)
		}

		l.Errorw("cannot create chain", "error", err)
		return chainsMap, nil, nil, true
	}

	err = s.SetWithExpiry(chainName, "true", 0)
	if err != nil {
		l.Errorw("unable to set chain name as true", "error", err)
	}

	l.Debugw("connected", "chainName", chainName)
	ctx, cancel := context.WithCancel(context.Background())
	rpcwatcher.Start(watcher, ctx)

	return chainsMap, watcher, cancel, false
}

func mapChains(c []cnsmodels.Chain) map[string]cnsmodels.Chain {
	ret := map[string]cnsmodels.Chain{}
	for _, cc := range c {
		ret[cc.ChainName] = cc
	}

	return ret
}

func endpoint(chainName string) string {
	return fmt.Sprintf("http://%s:26657", chainName)
}
