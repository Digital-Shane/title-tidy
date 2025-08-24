package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/Digital-Shane/title-tidy/internal/cmd"
	"github.com/Digital-Shane/title-tidy/internal/config"
	"github.com/Digital-Shane/title-tidy/internal/core"
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
	linkPath := flags.String("link", "", "Create links instead of renaming (optionally specify target directory)")
	linkType := flags.String("link-type", "auto", "Type of links to create: auto, hard, or soft")

	// Parse remaining arguments after the command
	if err := flags.Parse(os.Args[2:]); err != nil {
		fmt.Printf("Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Set flags and config in the command config
	cfg.InstantMode = *instant
	cfg.DeleteNFO = *noNFO
	cfg.DeleteImages = *noImages
	cfg.Config = formatConfig

	// Parse link mode settings
	if *linkPath != "" {
		// Set link target - if empty string, links will be created in current directory
		cfg.LinkTarget = *linkPath
		
		// If a path was provided, convert to absolute path
		if cfg.LinkTarget != "" {
			absPath, err := filepath.Abs(cfg.LinkTarget)
			if err != nil {
				fmt.Printf("Error resolving link target path: %v\n", err)
				os.Exit(1)
			}
			cfg.LinkTarget = absPath
		}
		
		// Parse link type
		switch *linkType {
		case "hard":
			cfg.LinkMode = core.LinkModeHard
		case "soft":
			cfg.LinkMode = core.LinkModeSoft
		case "auto":
			cfg.LinkMode = core.LinkModeAuto
		default:
			fmt.Printf("Invalid link type: %s (must be auto, hard, or soft)\n", *linkType)
			os.Exit(1)
		}
	}

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
	fmt.Printf("  title-tidy config    Configure custom naming formats\n")
	fmt.Printf("  title-tidy help      Show this help message\n\n")
	fmt.Printf("Options:\n")
	fmt.Printf("  -i, --instant          Apply renames immediately and exit\n")
	fmt.Printf("  --no-nfo               Delete NFO files during rename\n")
	fmt.Printf("  --no-img               Delete image files during rename\n")
	fmt.Printf("  --link [path]          Create links instead of renaming (optionally to target dir)\n")
	fmt.Printf("  --link-type TYPE       Type of links: auto (default), hard, or soft\n")
}
