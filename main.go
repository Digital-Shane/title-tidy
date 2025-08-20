package main

import (
	"flag"
	"fmt"
	"os"
	"slices"

	"github.com/Digital-Shane/title-tidy/internal/cmd"
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

	// Parse remaining arguments after the command
	if err := flags.Parse(os.Args[2:]); err != nil {
		fmt.Printf("Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Set flags in config
	cfg.InstantMode = *instant
	cfg.DeleteNFO = *noNFO
	cfg.DeleteImages = *noImages

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
	fmt.Printf("  title-tidy help      Show this help message\n\n")
	fmt.Printf("Options:\n")
	fmt.Printf("  -i, --instant          Apply renames immediately and exit\n")
	fmt.Printf("  --no-nfo               Delete NFO files during rename\n")
	fmt.Printf("  --no-img               Delete image files during rename\n")
}
