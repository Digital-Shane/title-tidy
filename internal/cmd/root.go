/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "title-tidy",
	Short: "A tool for renaming media files",
	Long: `title-tidy is a CLI tool that standardizes media file names for Jellyfin, Plex, and Emby.
It uses intelligent parsing to rename shows, seasons, episodes, and movies according to your
configured format templates.

The tool supports interactive preview mode and instant application mode, with optional
TMDB metadata lookup for enhanced naming accuracy.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

var (
	instant  bool
	noNfo    bool
	noImg    bool
	noSample bool
	linkPath string
)

func init() {
	// Global flags for all commands
	rootCmd.PersistentFlags().BoolVarP(&instant, "instant", "i", false, "Apply renames immediately without interactive preview")
	rootCmd.PersistentFlags().BoolVar(&noNfo, "no-nfo", false, "Delete NFO files during rename")
	rootCmd.PersistentFlags().BoolVar(&noImg, "no-img", false, "Delete image files during rename")
	rootCmd.PersistentFlags().BoolVar(&noSample, "no-sample", false, "Delete sample media files and folders during rename")
	rootCmd.PersistentFlags().StringVar(&linkPath, "link", "", "Create hard links in destination instead of renaming in place")
}
