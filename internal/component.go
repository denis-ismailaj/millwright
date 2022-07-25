package internal

import "time"

// Component represents a component that the internal is in charge of running.
type Component struct {
	// Config variables
	serviceName  string // serves as both container and DNS name
	runConfig    RunConfiguration
	dependencies []*Component
	ignore       bool // don't check health
	// Runtime variables
	containerID             string
	inspectPort             string // the host port the container introspection port is bound to
	status                  status
	lastSuccessfulHeartbeat time.Time
}

// RunConfiguration specifies how a Component can be run.
type RunConfiguration struct {
	DockerfilePath   string   // relative to build context
	BuildContextPath string   // absolute path
	Env              []string // in KEY=VALUE format
}

type status int32

// The statuses the Component may be in.
const (
	Unstarted status = iota
	Running
	Failed
)
