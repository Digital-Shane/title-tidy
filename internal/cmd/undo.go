package cmd

import (
	"fmt"

	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/Digital-Shane/title-tidy/internal/tui"
	"github.com/Digital-Shane/treeview"
	tea "github.com/charmbracelet/bubbletea"
)

// RunUndoCommand handles the undo subcommand
func RunUndoCommand() error {
	// Get session summaries for display
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
		// Create a node name with key session info
		nodeName := fmt.Sprintf("%s %s - %s (%d ops)",
			summary.Icon,
			summary.Session.Metadata.CommandArgs[0],
			summary.RelativeTime,
			summary.Session.Metadata.TotalOps)
		
		// Store the entire SessionSummary in the node
		node := treeview.NewNode(summary.Session.Metadata.SessionID, nodeName, summary)
		sessionNodes = append(sessionNodes, node)
	}
	
	// Create tree with SessionSummary as the generic type
	tree := treeview.NewTree(sessionNodes)
	model := tui.NewUndoModel(tree)
	
	// Launch the TUI
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}