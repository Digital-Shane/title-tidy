package cmd

import (
	"fmt"

	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/Digital-Shane/title-tidy/internal/tui"
	"github.com/Digital-Shane/treeview"
	"github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var undoCmd = &cobra.Command{
	Use:   "undo",
	Short: "Undo recent rename operations",
	Long: `Display recent rename operations and allow selective undo.
	
This command shows a list of previous rename sessions that can be undone,
allowing you to reverse changes made by title-tidy.`,
	RunE: runUndoCommand,
}

func runUndoCommand(cmd *cobra.Command, args []string) error {
	summaries, err := log.GetSessionSummaries()
	if err != nil {
		return fmt.Errorf("failed to read log sessions: %w", err)
	}

	if len(summaries) == 0 {
		fmt.Println("No operation sessions found to undo.")
		return nil
	}

	sessionNodes := make([]*treeview.Node[log.SessionSummary], 0, len(summaries))

	for _, summary := range summaries {
		nodeName := fmt.Sprintf("%s %s - %s (%d ops)",
			summary.Icon,
			summary.Session.Metadata.CommandArgs[0],
			summary.RelativeTime,
			summary.Session.Metadata.TotalOps)

		node := treeview.NewNode(summary.Session.Metadata.SessionID, nodeName, summary)
		sessionNodes = append(sessionNodes, node)
	}

	tree := treeview.NewTree(sessionNodes)
	model := tui.NewUndoModel(tree)

	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func init() {
	rootCmd.AddCommand(undoCmd)
}
