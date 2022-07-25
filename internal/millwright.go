package internal

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

var (
	reconcileCycleDelay    = 500 // Delay between each round of heartbeats (ms).
	reconcileFailedTimeout = 3   // The time between successful heartbeats required to mark a component as failed (ms).
)

// Millwright takes care of configuring, executing, and monitoring the other components.
type Millwright struct {
	cli        *client.Client
	components []*Component
}

// NewMillwright is a factory method for Millwright.
func NewMillwright() *Millwright {
	mw := &Millwright{}

	// Create Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal(err)
	}
	mw.cli = cli

	return mw
}

// Start launches all the components in the internal config.
func (mw *Millwright) Start(ctx context.Context) error {
	log.Info("Starting components.")
	for _, component := range mw.components {
		err := mw.LaunchComponentTree(ctx, component)
		if err != nil {
			return err
		}
	}
	return nil
}

// Reconcile continuously exchanges heartbeats with each of the components in order to
// detect potential failures.
func (mw *Millwright) Reconcile(ctx context.Context) {
	for {
		// Return if context has been cancelled.
		select {
		case <-ctx.Done():
			return
		default:
		}

		for _, component := range mw.components {
			if component.ignore || component.status == Failed {
				continue
			}

			ok := mw.SendHeartbeat(component)
			if ok {
				component.lastSuccessfulHeartbeat = time.Now()
				continue
			}

			// Check if component has been non-responsive for too long.
			timeSinceLastSuccessfulHeartbeat := time.Now().Sub(component.lastSuccessfulHeartbeat).Milliseconds()
			if timeSinceLastSuccessfulHeartbeat > int64(reconcileFailedTimeout) {
				component.status = Failed
				go mw.HandleFailedComponent(ctx, component)
			}
		}
		time.Sleep(time.Duration(reconcileCycleDelay) * time.Millisecond)
	}
}

// HandleFailedComponent relaunches a component that has failed and marks it as Running,
// so it can start being checked by Reconcile again.
func (mw *Millwright) HandleFailedComponent(ctx context.Context, component *Component) {
	_, err := mw.relaunchComponent(ctx, component)
	if err != nil {
		log.Errorf("ACTION REQUIRED: Failed component %s couldn't be relaunched: %v", component.serviceName, err)
	}

	// Set as running. If it still fails, Reconcile will flag it again.
	component.status = Running
}

// SendHeartbeat calls the introspection port for a component and returns weather the call succeeded.
func (mw *Millwright) SendHeartbeat(component *Component) bool {
	url := fmt.Sprintf("http://localhost:%s/debug/vars", component.inspectPort)
	get, err := http.Get(url)
	if err != nil {
		log.Infof("Heartbeat failed for %s.", component.serviceName)
		log.Error(err)
		return false
	}
	get.Body.Close()
	log.Infof("Successful heartbeat for %s.", component.serviceName)

	return true
}

// LaunchComponentTree is called recursively to walk down the dependencies of a component until the entire
// tree has been started.
func (mw *Millwright) LaunchComponentTree(ctx context.Context, component *Component) error {
	if component.status != Unstarted {
		// LaunchComponentTree has already traversed this component.
		return nil
	}

	// Check if component has been launched by another internal instance.
	id, ok := mw.getComponent(ctx, component)
	if ok {
		log.Infof("Component %s has already been launched.", component.serviceName)
		// Get its introspection port.
		if !component.ignore {
			// Find the component's introspection port and save it.
			inspectPort, err := mw.getIntrospectionPort(ctx, id)
			if err != nil {
				// Likely the container exists but is not in the correct network.
				// Let Reconcile deal with this.
				return nil
			}
			component.inspectPort = inspectPort
		}
		return nil
	}

	log.Infof("Checking dependencies for %s.", component.serviceName)

	// Ensure dependencies are satisfied.
	for _, dependency := range component.dependencies {
		err := mw.LaunchComponentTree(ctx, dependency)
		if err != nil {
			return err
		}
	}

	log.Infof("Launching %s.", component.serviceName)

	// Launch component.
	_, err := mw.launchComponent(ctx, component)
	if err != nil {
		return err
	}

	// Update component status.
	component.status = Running

	log.Infof("%s launched.", component.serviceName)

	return nil
}
