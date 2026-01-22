package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/cwel/kmux/internal/model"
	"github.com/cwel/kmux/internal/state"
	"github.com/cwel/kmux/internal/store"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Session state operations",
}

var sessionGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Output session save file as JSON",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := store.ValidateSessionName(name); err != nil {
			return err
		}

		st := store.DefaultStore()
		session, err := st.LoadSession(name)
		if err != nil {
			return fmt.Errorf("session not found: %s", name)
		}

		data, err := json.Marshal(session)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	},
}

var sessionSaveCmd = &cobra.Command{
	Use:   "save <name>",
	Short: "Save session from JSON on stdin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := store.ValidateSessionName(name); err != nil {
			return err
		}

		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}

		var session model.Session
		if err := json.Unmarshal(data, &session); err != nil {
			return fmt.Errorf("parse session: %w", err)
		}

		// Ensure name matches argument
		session.Name = name

		st := store.DefaultStore()
		return st.SaveSession(&session)
	},
}

var sessionDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete session save file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := store.ValidateSessionName(name); err != nil {
			return err
		}

		st := store.DefaultStore()
		return st.DeleteSession(name)
	},
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List sessions as JSON",
	RunE: func(cmd *cobra.Command, args []string) error {
		s := state.New()

		// Get local sessions (zmx + save files, no kitty — kitty is the caller's concern)
		sessions, _ := s.Sessions(true)

		data, err := json.Marshal(sessions)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	},
}

func init() {
	sessionCmd.AddCommand(sessionGetCmd)
	sessionCmd.AddCommand(sessionSaveCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
	sessionCmd.AddCommand(sessionListCmd)
	rootCmd.AddCommand(sessionCmd)
}
