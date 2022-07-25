package internal

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"path"
)

func configureComponents() []*Component {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("can't get cwd: %v", err)
	}

	demoware := &Component{
		serviceName: "demoware",
		runConfig: RunConfiguration{
			DockerfilePath:   "Dockerfile",
			BuildContextPath: path.Join(cwd, "demoware"),
			Env:              []string{},
		},
		ignore: true,
	}

	ingestion := &Component{
		serviceName: "ingestion",
		runConfig: RunConfiguration{
			DockerfilePath:   "Dockerfile",
			BuildContextPath: path.Join(cwd, "ingestion"),
			Env: []string{
				fmt.Sprintf("INGESTION_MAX_METRICS=%d", 10),
				fmt.Sprintf("INGESTION_BUFFER_SIZE=%d", 50),
				fmt.Sprintf("INGESTION_PORT=%d", 8080),
				fmt.Sprintf("METRICS_HOST=%s", demoware.serviceName),
				fmt.Sprintf("METRICS_PORT=%d", 8080),
				// These two should actually be in a secrets vault or sth else that is at least not committed to VCS
				fmt.Sprintf("METRICS_AUTH_USER=%s", "deadbeef"),
				fmt.Sprintf("METRICS_AUTH_PASS=%s", ""),
			},
		},
		dependencies: []*Component{demoware},
	}

	dispatcher := &Component{
		serviceName: "dispatcher",
		runConfig: RunConfiguration{
			DockerfilePath:   "./dispatcher/Dockerfile",
			BuildContextPath: ".",
			Env: []string{
				fmt.Sprintf("DISPATCHER_PORT=%d", 8080),
				fmt.Sprintf("DISPATCHER_MIN_BUFFER=%d", 20),
				fmt.Sprintf("DISPATCHER_INITIAL_BUFFER=%d", 50),
				fmt.Sprintf("DISPATCHER_MAX_METRICS=%d", 10),
				fmt.Sprintf("INGESTION_HOST=%s", ingestion.serviceName),
				fmt.Sprintf("INGESTION_PORT=%d", 8080),
			},
		},
		dependencies: []*Component{ingestion},
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

	kernelUpgradeHandler := &Component{
		serviceName: "handler_kernel_upgrade",
		runConfig: RunConfiguration{
			DockerfilePath:   "./handlerupgrade/Dockerfile",
			BuildContextPath: ".",
			Env: []string{
				fmt.Sprintf("HANDLER_POLL_DELAY=%d", 500),
				fmt.Sprintf("DISPATCHER_HOST=%s", dispatcher.serviceName),
				fmt.Sprintf("DISPATCHER_PORT=%d", 8080),
			},
		},
		dependencies: []*Component{dispatcher},
	}

	loadHandler := &Component{
		serviceName: "handler_load",
		runConfig: RunConfiguration{
			DockerfilePath:   "./handlerload/Dockerfile",
			BuildContextPath: ".",
			Env: []string{
				fmt.Sprintf("HANDLER_POLL_DELAY=%d", 500),
				fmt.Sprintf("DISPATCHER_HOST=%s", dispatcher.serviceName),
				fmt.Sprintf("DISPATCHER_PORT=%d", 8080),
			},
		},
		dependencies: []*Component{dispatcher},
	}

	return []*Component{
		dispatcher, ingestion, demoware, cpuUsageHandler, kernelUpgradeHandler, loadHandler,
	}
}
