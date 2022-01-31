package integration

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	dockertest "github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// spinUpTestChains is to be passed any number of test chains with given configuration options
// to be created as individual docker containers at the beginning of a test. It is safe to run
// in parallel tests as all created resources are independent of eachother
func spinUpTestChains(t *testing.T, pool *dockertest.Pool, network *dockertest.Network, testChains ...testChain) []testChain {
	var (
		resources = make([]*dockertest.Resource, 0, len(testChains))
		chains    = make([]testChain, len(testChains))

		// wg    sync.WaitGroup
		rchan = make(chan *dockertest.Resource, len(testChains))

		testsDone = make(chan struct{})
		contDone  = make(chan struct{})
	)

	var eg errgroup.Group
	// make each container and initialize the chains
	for i, tc := range testChains {
		tc := tc
		chains[i] = tc
		// wg.Add(1)
		eg.Go(func() error {
			return spinUpTestContainer(t, rchan, pool, tc, network)
		})
	}

	// wait for all containers to be created
	require.NoError(t, eg.Wait())

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
				// set node address in chain config
				chains[i].nodeAddress = r.GetIPInNetwork(network)
				t.Log(fmt.Sprintf("- [%s] CONTAINER AVAILABLE WITH IP: %s",
					chains[i].chainID, chains[i].nodeAddress))
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
	tc testChain, network *dockertest.Network) error {
	var (
		err error
		// debug    bool
		resource *dockertest.Resource
	)

	containerName := tc.chainID

	t.Log(fmt.Sprintf("- SETTING UP CONTAINER for [%s]", tc.chainID))

	// setup docker options
	dockerOpts := &dockertest.RunOptions{
		Name:         containerName,
		Repository:   containerName, // Name must match Repository
		Tag:          "latest",      // Must match docker default build tag
		ExposedPorts: []string{defaultRPCPort, defaultGRPCPort},
		Cmd: []string{
			tc.chainID,
			tc.accountInfo.seed,
			tc.accountInfo.primaryDenom,
		},
		Networks: []*dockertest.Network{network},
	}

	if err := removeTestContainer(pool, containerName); err != nil {
		return err
	}

	// create the proper docker image with port forwarding setup
	d, err := os.Getwd()
	if err != nil {
		return err
	}

	buildOpts := &dockertest.BuildOptions{
		Dockerfile: tc.dockerfile,
		ContextDir: path.Dir(d),
	}
	resource, err = pool.BuildAndRunWithBuildOptions(buildOpts, dockerOpts)
	if err != nil {
		return err
	}

	// TODO: need workaround to check node logs whether blocks started creating
	time.Sleep(10 * time.Second)

	t.Log(fmt.Sprintf("- [%s] SPUN UP IN CONTAINER %s from %s", tc.chainID,
		resource.Container.Name, resource.Container.Config.Image))

	rchan <- resource
	return nil
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

func spinRelayer(t *testing.T, pool *dockertest.Pool, network *dockertest.Network, args ...string) *dockertest.Resource {
	var (
		testsDone = make(chan struct{})
		contDone  = make(chan struct{})
	)

	t.Log("- SETTING UP CONTAINER for [relayer]")
	dockerOpts := &dockertest.RunOptions{
		Name:       "relayer",
		Repository: "relayer", // Name must match Repository
		Tag:        "latest",  // Must match docker default build tag
		Cmd:        args,
		Networks:   []*dockertest.Network{network},
	}

	require.NoError(t, removeTestContainer(pool, "relayer"))

	// create the proper docker image with port forwarding setup
	d, err := os.Getwd()
	require.NoError(t, err)

	buildOpts := &dockertest.BuildOptions{
		Dockerfile: "integration/setup/Dockerfile.relayer",
		ContextDir: path.Dir(d),
	}
	resource, err := pool.BuildAndRunWithBuildOptions(buildOpts, dockerOpts)
	require.NoError(t, err)

	t.Log(fmt.Sprintf("- [%s] SPUN UP IN CONTAINER %s from %s", "relayer",
		resource.Container.Name, resource.Container.Config.Image))

	// start the wait for cleanup function
	go cleanUpTest(t, testsDone, contDone, []*dockertest.Resource{resource}, pool, []testChain{})

	// set the test cleanup function
	t.Cleanup(func() {
		testsDone <- struct{}{}
		<-contDone
	})

	return resource
}
