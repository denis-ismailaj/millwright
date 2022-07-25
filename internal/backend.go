package internal

import (
	"context"
	"errors"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
	"io"
	"os"
)

// getOrCreateNetwork checks if a network by the specified name exists or creates a new one.
// It returns the network ID or an error.
func (mw *Millwright) getOrCreateNetwork(ctx context.Context, name string) (string, error) {
	list, err := mw.cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(filters.Arg("name", name)),
	})
	if err != nil {
		return "", err
	}
	if len(list) > 0 {
		// Network already exists.
		return list[0].ID, nil
	}

	net, err := mw.cli.NetworkCreate(ctx, name, types.NetworkCreate{
		Driver: "bridge",
		Labels: map[string]string{"used-by": ctx.Value(labelKey).(string)},
	})
	if err != nil {
		return "", err
	}
	return net.ID, nil
}

// getComponent tries to find an existing container with the name of the component.
// It returns the container ID and an ok value.
func (mw *Millwright) getComponent(ctx context.Context, component *Component) (string, bool) {
	list, err := mw.cli.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(filters.Arg("name", component.serviceName)),
	})
	if err != nil || len(list) == 0 {
		return "", false
	}

	id := list[0].ID
	component.containerID = id

	// Update status
	component.status = Running

	return id, true
}

// launchComponent creates a new container representing the given component
// and attaches to the network with the given name.
// It returns the container ID or an error.
func (mw *Millwright) launchComponent(ctx context.Context, component *Component) (string, error) {
	// Create a tar of the build context folder.
	tar, err := archive.TarWithOptions(component.runConfig.BuildContextPath, &archive.TarOptions{})
	if err != nil {
		return "", err
	}

	// Build the component's image.
	res, err := mw.cli.ImageBuild(ctx, tar, types.ImageBuildOptions{
		Dockerfile:  component.runConfig.DockerfilePath,
		PullParent:  true,
		Remove:      true,
		ForceRemove: true,
		Tags:        []string{component.serviceName},
		Labels:      map[string]string{"used-by": ctx.Value(labelKey).(string)},
	})
	if err != nil {
		return "", err
	}
	io.Copy(os.Stdout, res.Body)
	defer res.Body.Close()

	// Bind the introspection port of the container to the host.
	exposedPorts, portBindings, _ := nat.ParsePortSpecs([]string{
		fmt.Sprintf("127.0.0.1::%v", ctx.Value(introspectionPortKey)),
	})
	hostConfig := &container.HostConfig{
		AutoRemove:   true, // remove container when it exits
		PortBindings: portBindings,
	}

	// Make the container join the specified network when run.
	networkName := ctx.Value(networkNameKey).(string)
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{networkName: {}},
	}

	// Create the container
	containerConfig := &container.Config{
		Image:        component.serviceName,
		Env:          component.runConfig.Env,
		ExposedPorts: exposedPorts,
		Labels:       map[string]string{"used-by": ctx.Value(labelKey).(string)},
	}
	cont, err := mw.cli.ContainerCreate(
		ctx,
		containerConfig,
		hostConfig,
		networkConfig,
		nil,
		component.serviceName,
	)
	if err != nil {
		return "", err
	}

	// Start the container
	if err := mw.cli.ContainerStart(ctx, cont.ID, types.ContainerStartOptions{}); err != nil {
		return "", err
	}

	id := cont.ID
	component.containerID = id

	if !component.ignore {
		// Find the component's introspection port and save it.
		inspectPort, err := mw.getIntrospectionPort(ctx, id)
		if err != nil {
			return "", nil
		}
		component.inspectPort = inspectPort
	}

	return id, nil
}

// relaunchComponent removes the current container for a component if it exists, and then launches it.
func (mw *Millwright) relaunchComponent(ctx context.Context, component *Component) (string, error) {
	// Ignoring error if no container currently exists.
	// There's also the case that it may exist but for some reason couldn't be removed with force.
	// That error is not handled here, but it will however present an error when we try to launch below.
	_ = mw.cli.ContainerRemove(ctx, component.containerID, types.ContainerRemoveOptions{Force: true})

	id, err := mw.launchComponent(ctx, component)
	if err != nil {
		return "", err
	}
	return id, nil
}

// getIntrospectionPort finds the host port the container introspection port is bound to.
// It returns the port or an error.
func (mw *Millwright) getIntrospectionPort(ctx context.Context, containerID string) (string, error) {
	inspect, err := mw.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}
	internalPort := nat.Port(fmt.Sprintf("%v/tcp", ctx.Value(introspectionPortKey)))
	ports := inspect.NetworkSettings.Ports[internalPort]
	if len(ports) == 0 {
		return "", errors.New("can't get introspection port")
	}
	introspectionPort := ports[0].HostPort
	return introspectionPort, nil
}
