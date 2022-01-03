package integration

import (
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	dockertest "github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
)

// spinUpTestChains is to be passed any number of test chains with given configuration options
// to be created as individual docker containers at the beginning of a test. It is safe to run
// in parallel tests as all created resources are independent of eachother
func spinUpTestChains(t *testing.T, testChains ...testChain) []testChain {
	var (
		resources = make([]*dockertest.Resource, 0, len(testChains))
		chains    = make([]testChain, len(testChains))

		wg    sync.WaitGroup
		rchan = make(chan *dockertest.Resource, len(testChains))

		testsDone = make(chan struct{})
		contDone  = make(chan struct{})
	)

	// uses a sensible default on windows (tcp/http) and linux/osx (socket)
	pool, err := dockertest.NewPool("")
	if err != nil {
		require.NoError(t, fmt.Errorf("could not connect to docker at %s: %w", pool.Client.Endpoint(), err))
	}

	// make each container and initialize the chains
	for i, tc := range testChains {
		chains[i] = tc
		wg.Add(1)
		go spinUpTestContainer(t, rchan, pool, &wg, chains[i])
	}

	// wait for all containers to be created
	wg.Wait()

	// read all the containers out of the channel
	for i := 0; i < len(chains); i++ {
		r := <-rchan
		resources = append(resources, r)
	}

	// assign resource to specific chain
	for i := 0; i < len(chains); i++ {
		for _, r := range resources {
			if strings.Contains(r.Container.Name, chains[i].chainID) {
				chains[i].resource = r
				ports := r.Container.NetworkSettings.Ports[dc.Port("26657/tcp")]
				require.Greater(t, len(ports), 0)
				chains[i].rpcPort = ports[0].HostPort
				t.Log(fmt.Sprintf("- [%s] CONTAINER AVAILABLE AT PORT %s", chains[i].chainID, chains[i].rpcPort))
				break
			}
		}
	}

	// close the channel
	close(rchan)

	// start the wait for cleanup function
	go cleanUpTest(t, testsDone, contDone, resources, pool, chains)

	// set the test cleanup function
	t.Cleanup(func() {
		testsDone <- struct{}{}
		<-contDone
	})

	// return the chains
	return chains
}

// spinUpTestContainer spins up a test container with the given configuration
// A docker image is built for each chain using its provided configuration.
// This image is then ran using the options set below.
func spinUpTestContainer(t *testing.T, rchan chan<- *dockertest.Resource, pool *dockertest.Pool,
	wg *sync.WaitGroup, tc testChain) {
	defer wg.Done()
	var (
		err error
		// debug    bool
		resource *dockertest.Resource
	)

	containerName := tc.chainID

	// setup docker options
	dockerOpts := &dockertest.RunOptions{
		Name:         containerName,
		Repository:   containerName, // Name must match Repository
		Tag:          "latest",      // Must match docker default build tag
		ExposedPorts: []string{"26657"},
		Cmd: []string{
			tc.chainID,
			tc.accountInfo.seed,
			tc.accountInfo.denom,
		},
	}

	require.NoError(t, removeTestContainer(pool, containerName))

	// create the proper docker image with port forwarding setup
	d, err := os.Getwd()
	require.NoError(t, err)

	buildOpts := &dockertest.BuildOptions{
		Dockerfile: tc.dockerfile,
		ContextDir: path.Dir(d),
	}
	resource, err = pool.BuildAndRunWithBuildOptions(buildOpts, dockerOpts)
	require.NoError(t, err)

	// TODO: need workaround to check node logs whether blocks started creating
	time.Sleep(10 * time.Second)

	t.Log(fmt.Sprintf("- [%s] SPUN UP IN CONTAINER %s from %s", tc.chainID,
		resource.Container.Name, resource.Container.Config.Image))

	rchan <- resource
}

func removeTestContainer(pool *dockertest.Pool, containerName string) error {
	containers, err := pool.Client.ListContainers(dc.ListContainersOptions{
		All: true,
		Filters: map[string][]string{
			"name": {containerName},
		},
	})
	if err != nil {
		return fmt.Errorf("error while listing containers with name %s %w", containerName, err)
	}

	if len(containers) == 0 {
		return nil
	}

	err = pool.Client.RemoveContainer(dc.RemoveContainerOptions{
		ID:            containers[0].ID,
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil {
		return fmt.Errorf("error while removing container with name %s %w", containerName, err)
	}

	return nil
}

// cleanUpTest is called as a goroutine to wait until the tests have completed and
// cleans up the docker containers
func cleanUpTest(t *testing.T, testsDone <-chan struct{}, contDone chan<- struct{}, resources []*dockertest.Resource,
	pool *dockertest.Pool, chains []testChain) {
	// block here until tests are complete
	<-testsDone

	// remove all the docker containers
	for _, r := range resources {
		if err := pool.Purge(r); err != nil {
			require.NoError(t, fmt.Errorf("could not purge container %s: %w", r.Container.Name, err))
		}
		c := getLoggingChain(chains, r)
		if c.chainID != "" {
			t.Log(fmt.Sprintf("- [%s] SPUN DOWN CONTAINER %s from %s", c.chainID, r.Container.Name,
				r.Container.Config.Image))
		}
	}

	// Notify the other side that we have deleted the docker containers
	contDone <- struct{}{}
}

func getLoggingChain(chns []testChain, rsr *dockertest.Resource) testChain {
	for _, c := range chns {
		if strings.Contains(rsr.Container.Name, c.chainID) {
			return c
		}
	}
	return testChain{}
}
