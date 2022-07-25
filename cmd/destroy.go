package cmd

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	label string
)

func init() {
	destroyCmd.Flags().StringVarP(&label, "label", "l", "used-by=millwright", "Label used for resources.")
	RootCmd.AddCommand(destroyCmd)
}

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Args:  cobra.NoArgs,
	Short: "Removes all resources created for this test task.",
	Run:   destroy,
}

func destroy(*cobra.Command, []string) {
	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	// Filter for the used-by label
	labelFilter := filters.NewArgs(
		filters.Arg("label", label),
	)

	// Find and remove containers
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{Filters: labelFilter})
	if err != nil {
		log.Fatal(err)
	}
	for _, container := range containers {
		err := cli.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			log.Fatal(err)
		}
	}

	// Find and remove images
	images, err := cli.ImageList(ctx, types.ImageListOptions{Filters: labelFilter})
	if err != nil {
		log.Fatal(err)
	}
	for _, image := range images {
		_, err := cli.ImageRemove(ctx, image.ID, types.ImageRemoveOptions{Force: true})
		if err != nil {
			log.Fatal(err)
		}
	}

	// Find and remove networks
	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{Filters: labelFilter})
	if err != nil {
		log.Fatal(err)
	}
	for _, network := range networks {
		err := cli.NetworkRemove(ctx, network.ID)
		if err != nil {
			log.Fatal(err)
		}
	}
}
