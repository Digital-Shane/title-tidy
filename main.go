package main

import (
	"flag"
	"fmt"
	"os"
	"slices"

	"github.com/Digital-Shane/title-tidy/internal/cmd"
	"github.com/Digital-Shane/title-tidy/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	command := os.Args[1]

	configs := map[string]cmd.CommandConfig{
		"shows":    cmd.ShowsCommand,
		"seasons":  cmd.SeasonsCommand,
		"episodes": cmd.EpisodesCommand,
		"movies":   cmd.MoviesCommand,
	}
	helpKeywords := []string{"help", "--help", "-h"}

	// Handle help command
	if slices.Contains(helpKeywords, command) {
		printUsage()
		return
	}

	// Handle config command
	if command == "config" {
		if err := cmd.RunConfigCommand(os.Args[2:]); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle undo command
	if command == "undo" {
		if err := cmd.RunUndoCommand(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Load config once for all operations
	formatConfig, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Run a rename command
	cfg, ok := configs[command]
	if !ok {
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}

	// Parse flags for the command
	flags := flag.NewFlagSet(command, flag.ExitOnError)
	instant := flags.Bool("i", false, "Apply renames immediately without interactive preview")
	flags.BoolVar(instant, "instant", false, "Apply renames immediately without interactive preview")
	noNFO := flags.Bool("no-nfo", false, "Delete NFO files during rename")
	noImages := flags.Bool("no-img", false, "Delete image files during rename")
	noSamples := flags.Bool("no-sample", false, "Delete sample media files and folders during rename")
	linkPath := flags.String("link", "", "Create hard links in destination instead of renaming in place")

	// Parse remaining arguments after the command
	if err := flags.Parse(os.Args[2:]); err != nil {
		fmt.Printf("Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Validate link path if provided
	if *linkPath != "" {
		info, err := os.Stat(*linkPath)
		if err != nil {
			fmt.Printf("Error: Link destination does not exist: %v\n", err)
			os.Exit(1)
		}
		if !info.IsDir() {
			fmt.Printf("Error: Link destination must be a directory\n")
			os.Exit(1)
		}
	}

	// Set flags and config in the command config
	cfg.InstantMode = *instant
	cfg.DeleteNFO = *noNFO
	cfg.DeleteImages = *noImages
	cfg.DeleteSamples = *noSamples
	cfg.Config = formatConfig
	cfg.LinkPath = *linkPath
	cfg.Command = command
	cfg.CommandArgs = os.Args[2:]

	if err := cmd.RunCommand(cfg); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf("title-tidy - A tool for renaming media files\n\n")
	fmt.Printf("Usage:\n")
	fmt.Printf("  title-tidy shows     Rename TV show files and folders\n")
	fmt.Printf("  title-tidy seasons   Rename season folders and episodes within\n")
	fmt.Printf("  title-tidy episodes  Rename episode files in current directory\n")
	fmt.Printf("  title-tidy movies    Rename movie files and folders\n")
	fmt.Printf("  title-tidy undo      Undo recent rename operations\n")
	fmt.Printf("  title-tidy config    Configure custom naming formats\n")
	fmt.Printf("  title-tidy help      Show this help message\n\n")
	fmt.Printf("Options:\n")
	fmt.Printf("  -i, --instant          Apply renames immediately and exit\n")
	fmt.Printf("  --no-nfo               Delete NFO files during rename\n")
	fmt.Printf("  --no-img               Delete image files during rename\n")
	fmt.Printf("  --no-sample            Delete sample media files and folders during rename\n")
	fmt.Printf("  --link <path>          Create hard links in destination instead of renaming\n")
}
