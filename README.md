# Millwright

Millwright is a tool for configuring, executing, and monitoring multi-container Docker projects.

Millwright uses the Docker Client SDK as a backend to handle builds, execution, and networking.
It exchanges heartbeats with the components to detect failures and handles them automatically.

To use it, first build millwright. If you're in the project root you can run

    go build -o mw

and you'll have a binary named `mw` ready to use.

**Note: The user that runs `millwright` commands must have permission to access the docker engine socket
(e.g. by being in the `docker` user group. For quick fix: `sudo usermod -aG docker $LOGNAME`).**

#### Configuration

Millwright is configured programmatically using the `configureComponents` function in `internal/config.go`.

Component configurations and dependencies are all specified here. Here's a shortened example of a config.

    func configureComponents() []*Component {
       dispatcher := &Component{
		serviceName: "dispatcher",
		runConfig: RunConfiguration{
			DockerfilePath:   "./dispatcher/Dockerfile",
			BuildContextPath: ".",
			Env: []string{
				fmt.Sprintf("DISPATCHER_MIN_BUFFER=%d", 20),
			},
		},
		dependencies: []*Component{},
        }
    
        cpuUsageHandler := &Component{
            serviceName: "handler_cpu_usage",
            runConfig: RunConfiguration{
                DockerfilePath:   "./handlercpu/Dockerfile",
                BuildContextPath: ".",
                Env: []string{
                    fmt.Sprintf("HANDLER_POLL_DELAY=%d", 500),
                    fmt.Sprintf("DISPATCHER_HOST=%s", dispatcher.serviceName),
                    fmt.Sprintf("DISPATCHER_PORT=%d", 8080),
                },
            },
            dependencies: []*Component{dispatcher},
        }

        return []*Component{dispatcher, cpuUsageHandler}
	}

#### Start

Millwright can create the infrastructure and start monitoring it using this command:

    mw start [--force]

**Due to the modules not being published this command has to be run in the root project path,
or otherwise it won't be able to find the modules. (This only affects `start`, the other commands can be run
from anywhere)**

Note that when a millwright is terminated the components can continue running as usual.
When started, a millwright checks existing components and knows to only launch the components
that are missing or that have failed.
It can also take care of components that are running but are attached to the wrong network.

Multiple millwrights can be started in order to have fault tolerance.
By default, when running `mw start` the new millwright waits until any existing millwrights have exited.
This behavior is enabled by using [Derailleur](https://github.com/denis-ismailaj/derailleur).

To force a millwright to start immediately without waiting you can supply the `--force` flag.
If another millwright is already running it will be forced to quit automatically.

Note that coordination only applies when you _start_ a millwright.
The other commands (described below) are executed right away without starting a new millwright.

#### Inspect

Millwright can be used to manually inspect what a component is serving on its introspection endpoint using this
command:

    mw inspect <component_name>

This will return a JSON object that contains the default `expvar` published variables, and for the payload handlers, it
also shows their internal state.

#### Kill

Millwright can be used to forcibly stop the container for a specific component using this command:

    mw kill <component_name>

Note that no output means it finished successfully.

#### Destroy

Millwright can bring down and forcibly remove all the resources it has created using the following command:

    mw destroy

Under the hood this relies on the fact that the label `used-by=millwright` is added to all the resources.

## Testing

The following command run the unit tests:

    go test ./...

There is also a script that tests millwright commands in various situations. When in the project root, you can run:

    cd internal && ./millwright_test.sh

## Assumptions

- Partially synchronous network model: messages may be dropped or reordered but can eventually get through if retried,
  packets are not spoofed/modified in transit.
- Crash-stop failure model: components can handle some failures on their own without crashing, but if they do crash they
  stop executing forever.
- Non-Byzantine behavior: processes don't deviate from their algorithms and are assumed to be non-malicious.

## Expanding further

Here I will discuss how my implementation of this project could be extended for a real-world system.

### Execution environment

The real-world equivalent for millwright would be something that can run on multiple nodes, not just locally.

### Service discovery

In order to find the (ever-changing) IPs of the containers I am currently making use of the fact that Docker creates
DNS names for the services in a user-defined network.

In a real system I would have launched a DNS-SD server that all containers are configured to use for DNS queries.
(It would fall back to an external DNS server for queries unrelated to service discovery)
When a container is started, it would register itself in the DNS-SD server.
This would also make it possible to do round-robin load balancing on the different containers for a given service,
or if necessary, to let all containers for a given service find each other (this will be useful below).

### Scaling

##### Coordination

Millwrights are already "distributed", but the coordination method that they use requires access to a shared filesystem.
Even if a distributed filesystem (such as Ceph) were used, `inotify` wouldn't work there so the current implementation
would still fail.

One way to make this work would be to use a separate coordination service (e.g. like Zookeeper, from which the 
`Derailleur` implementation was inspired).

##### Management

Currently, millwright considers a component to have a one-to-one relationship with a container.
This approach would be changed to a component's _instance_ having a separate status, while a component in itself
is a _group_ of these instances with a _desired_ and _current_ scale.

#### Observability

Currently, I'm just using `expvar` to be able to check health and to manually inspect internal states.
In a production system, I could still have a separate health check endpoint for the millwright to use, but for proper
observability I would use a solution that can collect logs, resources usages, etc... from all the components,
persist them (perhaps in a time-series database), and visualize them all in one place.

#### Auto-scaling

By having access to resource usages, millwright could do auto-scaling of components in order to handle load spikes,
or to save processing power when not needed.

Also, if component instances were aware of their own resource usage and limits, in order to avoid failures like for
example an OOM kill, a threshold near the resource limit could be added, during which a component would not process
any new requests but only respond with a failure status code (such as 503) that would make the corresponding consumers
do exponential backoff until auto-scaling kicks in.

#### Configuration changes

Minimal configuration changes can (sort of) be done with the current implementation. If the millwright config is changed
and a millwright with this new config is built and run with `--force` it would replace the current millwright and try to
reach a state it is happy with. However, this is currently quite limited and a proper solution would have to be
implemented in a production system.

#### Versioning

In the current implementation, a component can be redeployed after a change by doing `mw kill <component>` which
would bring it down and then the millwright which is running would detect that as a failure and build it again (this time
with the new version). This, however, is not enough because in a production system you would want to make such changes
without any downtime. Currently, millwright has no notion of versioning for the components so that would have to
change.
