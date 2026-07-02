package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

	const Version = "1.1.0"

func main() {
	if len(os.Args) > 1 {
		arg := strings.ToLower(os.Args[1])
		switch arg {
		case "--update", "-u", "update":
			checkUpdate()
			return
		case "--serve", "-s", "serve":
			configPath := FindConfigPath()
			cfg, err := LoadConfig(configPath)
			if err != nil {
				fmt.Printf("Config error: %v\n", err)
				os.Exit(1)
			}
			runDirectServer(cfg)
			return
		case "--support", "--donate", "support":
			printSupport()
			return
		case "--version", "-v", "version":
			fmt.Printf("AnyRouter v%s\n", Version)
			return
		case "--help", "-h", "help":
			printHelp()
			return
		}
	}

	configPath := FindConfigPath()
	cfg, err := LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Config error: %v\n", err)
		WriteDefaultConfig(configPath)
		cfg, _ = LoadConfig(configPath)
	}

	p := tea.NewProgram(initialModel(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func checkUpdate() {
	fmt.Print("\033[36m")
	fmt.Println(RenderBanner())
	fmt.Print("\033[0m")

	resp, err := http.Get("https://api.github.com/repos/anyrouter/cli/releases/latest")
	if err != nil {
		fmt.Printf("  Could not check for updates: %v\n", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		fmt.Println("  Could not parse release info.")
		return
	}
	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(Version, "v")

	fmt.Printf("  Current version: v%s\n", Version)
	fmt.Printf("  Latest version:  v%s\n", latest)

	if compareVersions(latest, current) > 0 {
		fmt.Println()
		fmt.Print("\033[33m")
		fmt.Println("  ⬆ Update available!")
		fmt.Print("\033[0m")
		fmt.Println()
		fmt.Println("  Run to upgrade:")
		fmt.Println("    powershell -c \"irm https://anyrouter.planixx.com/scripts/install.ps1 | iex\"")
	} else {
		fmt.Println()
		fmt.Println("  ✓ You are up to date!")
	}
	fmt.Println()
}

func printHelp() {
	fmt.Println(RenderBanner())
	fmt.Println("Usage:")
	fmt.Println("  anyrouter           Start interactive TUI")
	fmt.Println("  anyrouter --serve   Start server directly")
	fmt.Println("  anyrouter --support Show support information")
	fmt.Println("  anyrouter --version Show version")
	fmt.Println("  anyrouter --update  Check for updates")
	fmt.Println("  anyrouter --help    Show this help")
	fmt.Println()
}

func printSupport() {
	fmt.Print("\033[36m")
	fmt.Println(RenderBanner())
	fmt.Print("\033[0m")
	fmt.Println()
	fmt.Println("  AnyRouter is open-source and free to use.")
	fmt.Println()
	fmt.Println("  If you find this tool useful, consider supporting")
	fmt.Println("  the project:")
	fmt.Println()
	fmt.Println("    GitHub Sponsors:  https://github.com/sponsors/anyrouter")
	fmt.Println("    Buy Me a Coffee:  https://ko-fi.com/anyrouter")
	fmt.Println("    Paypal:           https://paypal.me/anyrouter")
	fmt.Println()
	fmt.Println("  Your support helps maintain and improve AnyRouter.")
	fmt.Println()
}

func runDirectServer(cfg *Config) {
	fmt.Print("\033[36m")
	fmt.Println(RenderBanner())
	fmt.Print("\033[0m")

	srv := NewProxyServer(cfg)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", srv.handleChatCompletions)
	mux.HandleFunc("/v1/messages", srv.handleMessages)
	mux.HandleFunc("/v1/models", srv.handleModels)
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/debug/routes", srv.handleDebugRoutes)
	mux.HandleFunc("/", srv.handleNotFound)
	srv.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: corsMiddleware(mux),
	}

	fmt.Printf("  AnyRouter running on http://%s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("  Endpoints: POST /v1/chat/completions, POST /v1/messages\n")
	fmt.Printf("  %d providers - Smart failover - Multi-key rotation\n\n",
		countEnabled(cfg))

	if err := srv.Start(); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}

func countEnabled(cfg *Config) int {
	n := 0
	for _, p := range cfg.Providers {
		if p.Enabled {
			n++
		}
	}
	return n
}
