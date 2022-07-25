package cmd

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(killCmd)
}

var killCmd = &cobra.Command{
	Use:   "kill",
	Short: "Removes a component's container by force.",
	Args:  cobra.ExactArgs(1),
	Run:   kill,
}

func kill(_ *cobra.Command, args []string) {
	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// The name of the component to kill
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

	// Remove container forcibly
	err = cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
	if err != nil {
		log.Fatal(err)
	}
}
