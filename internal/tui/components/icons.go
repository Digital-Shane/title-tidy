package components

import (
	"os"
	"runtime"
)

// Icon sets for different terminal capabilities
var (
	// EmojiIcons for modern terminals with full Unicode support
	EmojiIcons = map[string]string{
		// Media types
		"tv":        "📺",
		"show":      "📺",
		"movie":     "🎬",
		"episode":   "🎬",
		"season":    "📁",
		"seasons":   "📁",
		"episodes":  "🎬",
		"video":     "🎥",
		"film":      "🎬",
		"moviefile": "🎥",

		// File types
		"folder":    "📁",
		"document":  "📄",
		"subtitles": "📄",
		"default":   "📄",

		// Status indicators
		"success":    "✅",
		"error":      "❌",
		"delete":     "❌",
		"check":      "✅",
		"needrename": "✓",
		"nochange":   "=",
		"virtual":    "➕",
		"link":       "🔗",
		"unknown":    "❓",

		// UI elements
		"stats":    "📊",
		"chart":    "📊",
		"chip":     "🧠",
		"title":    "📺",
		"calendar": "📅",
		"key":      "🔑",
		"globe":    "🌐",
		"arrows":   "↑↓←→",
	}

	// ASCIIIcons for SSH sessions, Windows terminals, and fallback
	ASCIIIcons = map[string]string{
		// Media types
		"tv":        "[TV]",
		"show":      "[TV]",
		"movie":     "[M]",
		"episode":   "[E]",
		"season":    "[S]",
		"seasons":   "[D]",
		"episodes":  "[E]",
		"video":     "[V]",
		"film":      "[F]",
		"moviefile": "[F]",

		// File types
		"folder":    "[D]",
		"document":  "[F]",
		"subtitles": "[S]",
		"default":   "[ ]",

		// Status indicators
		"success":    "[v]",
		"error":      "[!]",
		"delete":     "[x]",
		"check":      "[✓]",
		"needrename": "[+]",
		"nochange":   "[=]",
		"virtual":    "[+]",
		"link":       "[→]",
		"unknown":    "[?]",

		// UI elements
		"stats":    "[*]",
		"chart":    "[#]",
		"chip":     "[T]",
		"title":    "[TV]",
		"calendar": "[C]",
		"key":      "[K]",
		"globe":    "[G]",
		"arrows":   "^v<>",
	}
)

// SelectIcons chooses the best icon set based on terminal capabilities
func SelectIcons() map[string]string {
	if IsLimitedTerminal() {
		return ASCIIIcons
	}
	return EmojiIcons
}

// IsLimitedTerminal detects environments where ASCII icons are better than emoji
func IsLimitedTerminal() bool {
	// SSH sessions typically have limited emoji support
	if isSshSession() {
		return true
	}

	// Windows terminals (especially PowerShell) often render emoji poorly
	if runtime.GOOS == "windows" {
		return true
	}

	return false
}

func isSshSession() bool {
	return os.Getenv("SSH_CLIENT") != "" ||
		os.Getenv("SSH_TTY") != "" ||
		os.Getenv("SSH_CONNECTION") != ""
}
