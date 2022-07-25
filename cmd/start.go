package cmd

import (
	"context"
	"github.com/denis-ismailaj/coordinator"
	"github.com/denis-ismailaj/millwright/internal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"path"
	"syscall"
)

var (
	force    bool
	startCmd = &cobra.Command{
		Use:   "start",
		Short: "Start the data processing pipeline and monitor it.",
		Run:   start,
	}
)

func init() {
	startCmd.Flags().BoolVar(&force, "force", false, "Interrupt preceding millwrights.")
	RootCmd.AddCommand(startCmd)
}

func start(*cobra.Command, []string) {
	// Create main context with cancellation
	ctx, cancelFn := context.WithCancel(context.Background())
	// Handle signals
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT)
		s := <-sigCh
		log.Errorf("terminating due to signal %v", s)
		cancelFn()
	}()

	// Create a coordinator instance
	coordinator := coordination.Coordinator{
		Dir: path.Join(os.TempDir(), "millwright"),
	}

	// Create a wait file for this internal
	file, err := coordinator.CreateWaitFile()
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(file.Name())

	// Check if another internal has dethroned us.
	ownFileChan := make(chan error)
	ownWatcher := coordinator.WaitForFile(coordinator.FilePath, ownFileChan)
	defer ownWatcher.Close()
	go func() {
		<-ownFileChan
		log.Error("The wait file of this queuer was forcibly removed. Quitting.")
		cancelFn()
	}()

	// Wait for preceding millwrights to quit or force them to quit.
	if force {
		log.Info("--force enabled, preceding millwrights will be removed.")
		err := coordinator.CutInLine()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		coordinator.WaitInLine(ctx)
	}

	// Check if context has been cancelled.
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Start internal
	log.Info("Starting internal.")
	internal.StartMillwright(ctx)
}
