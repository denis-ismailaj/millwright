package internal

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
)

type key int

// Keys for the values passed in context.
const (
	introspectionPortKey key = iota
	networkNameKey
	labelKey
)

// StartMillwright configures, creates, and launches a new internal instance.
func StartMillwright(ctx context.Context) {
	// Create new internal instance
	mw := NewMillwright()

	// I'm not a fan of adding values to context but these would transit a lot of function signatures,
	// so I think it's appropriate here.
	introspectionPort := 8089
	ctx = context.WithValue(ctx, introspectionPortKey, introspectionPort)
	networkName := "millwright-bridge"
	ctx = context.WithValue(ctx, networkNameKey, networkName)
	// This will be used to clean up resources.
	ctx = context.WithValue(ctx, labelKey, "millwright")

	// Get configuration from config.go and make sure it is valid.
	components := configureComponents()
	err := checkConfiguration(components)
	if err != nil {
		log.Fatal(err)
	}

	// Add the INTROSPECTION_PORT env var to the components.
	for _, component := range components {
		if component.ignore {
			continue
		}
		component.runConfig.Env = append(component.runConfig.Env,
			fmt.Sprintf("INTROSPECTION_PORT=%d", introspectionPort),
		)
	}

	// Save component configuration to internal.
	mw.components = components

	// Return if context has been cancelled.
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Create a bridge network where all components will be attached.
	_, err = mw.getOrCreateNetwork(ctx, networkName)
	if err != nil {
		log.Fatalf("can't create network: %v", err)
	}

	// Start components
	err = mw.Start(ctx)
	if err != nil {
		// We can handle components failing, but if they can't start at all
		// then this is likely a configuration or user issue that the internal can't solve.
		log.Fatalf("can't start components: %v", err)
	}

	// Start reconciliation loop
	mw.Reconcile(ctx)
}

// checkConfiguration is used to ensure a component configuration is valid.
// Currently, it only (inefficiently) checks of direct cyclic dependencies.
func checkConfiguration(components []*Component) error {
	for _, component := range components {
		for _, dependency := range component.dependencies {
			for _, d := range dependency.dependencies {
				if d == component {
					return fmt.Errorf(
						"cyclic dependency found: %s and %s",
						component.serviceName, dependency.serviceName,
					)
				}
			}
		}
	}
	return nil
}
