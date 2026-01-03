package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/cwel/kmux/internal/config"
	"github.com/cwel/kmux/internal/daemon/client"
	"github.com/cwel/kmux/internal/daemon/server"
	"github.com/sevlyar/go-daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Daemon management",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		foreground, _ := cmd.Flags().GetBool("foreground")

		socketPath := config.SocketPath()
		dataDir := config.DataDir()
		socketDir := filepath.Dir(socketPath)
		pidFile := filepath.Join(socketDir, "kmux.pid")
		logFile := filepath.Join(dataDir, "daemon.log")

		// Ensure directories exist
		os.MkdirAll(socketDir, 0700)
		os.MkdirAll(dataDir, 0700)

		// Clean up stale state: if PID file exists but socket doesn't, daemon is dead
		if _, err := os.Stat(pidFile); err == nil {
			if _, err := os.Stat(socketPath); os.IsNotExist(err) {
				// PID exists but socket doesn't - stale state
				os.Remove(pidFile)
			}
		}

		if foreground {
			// Run in foreground with signal handling
			fmt.Printf("Starting daemon (foreground) on %s\n", socketPath)
			srv := server.New(socketPath, dataDir)

			// Handle signals
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				srv.Stop()
			}()

			return srv.Start()
		}

		// Daemonize
		cntxt := &daemon.Context{
			PidFileName: pidFile,
			PidFilePerm: 0644,
			LogFileName: logFile,
			LogFilePerm: 0640,
			WorkDir:     dataDir,
			Umask:       027,
		}

		d, err := cntxt.Reborn()
		if err != nil {
			return fmt.Errorf("daemonize: %w", err)
		}
		if d != nil {
			// Parent process - daemon started successfully
			return nil
		}
		defer cntxt.Release()

		// Child process (daemon) - set up signal handling
		srv := server.New(socketPath, dataDir)

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			srv.Stop()
		}()

		return srv.Start()
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(config.SocketPath())

		if c.IsRunning() {
			fmt.Println("Daemon is running")
			return nil
		}

		fmt.Println("Daemon is not running")
		os.Exit(1)
		return nil
	},
}

var daemonKillCmd = &cobra.Command{
	Use:   "kill-server",
	Short: "Stop the daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.New(config.SocketPath())

		if !c.IsRunning() {
			fmt.Println("Daemon is not running")
			return nil
		}

		if err := c.Shutdown(); err != nil {
			return err
		}

		fmt.Println("Daemon stopped")
		return nil
	},
}

func init() {
	daemonStartCmd.Flags().BoolP("foreground", "f", false, "Run in foreground")

	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonKillCmd)
	rootCmd.AddCommand(daemonCmd)
}
