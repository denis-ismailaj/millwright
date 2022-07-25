package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
)

var (
	port int
)

func init() {
	inspectCmd.Flags().IntVarP(&port, "port", "p", 8089, "Internal introspection port.")
	RootCmd.AddCommand(inspectCmd)
}

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Displays the published variables for a component.",
	Args:  cobra.ExactArgs(1),
	Run:   inspect,
}

func inspect(_ *cobra.Command, args []string) {
	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Name of the component to inspect
	name := args[0]

	// Find the container for the component
	list, err := cli.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(
			filters.Arg("name", name),
		),
	})
	if err != nil {
		log.Fatal(err)
	}
	if len(list) == 0 {
		log.Fatalf("container for component %s not found", name)
	}
	containerID := list[0].ID

	// Find the host port the internal introspection port is bound to.
	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		log.Fatal(err)
	}
	internalPort := nat.Port(fmt.Sprintf("%d/tcp", port))
	ports := inspect.NetworkSettings.Ports[internalPort]
	if len(ports) == 0 {
		log.Fatal(errors.New("can't get introspection port"))
	}
	introspectionPort := ports[0].HostPort

	// Call the introspection endpoint
	url := fmt.Sprintf("http://localhost:%s/debug/vars", introspectionPort)
	response, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	// Output the response
	bytea, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	// Using fmt here because logrus for some reason uses stderr for INFO output,
	// and so I can't mute stdout and display only stderr when testing.
	fmt.Printf(string(bytea))
}
