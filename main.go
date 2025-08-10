// main.go
package main

import (
	"fmt"
	"log"
	"os"
	"io"
	"encoding/json"
	"strings"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		initializeWorkspace()
	case "generate":
		if len(os.Args) < 3 {
			fmt.Println("Usage: api-man generate <openapi-spec.yaml>")
			os.Exit(1)
		}
		generateFromOpenAPI(os.Args[2])
	case "run":
		if len(os.Args) < 4 {
			fmt.Println("Usage: api-man run <request-path> <environment>")
			fmt.Println("Example: api-man run users/get-users dev")
			os.Exit(1)
		}
		runRequest(os.Args[2], os.Args[3])
	case "list":
		listRequests()
	case "envs":
		listEnvironments()
	case "tui":
		if len(os.Args) < 3 {
			fmt.Println("Usage: api-man tui <openapi-spec.yaml>")
			os.Exit(1)
		}
		runTUI(os.Args[2])
	case "body":
		handleBodyCommand()
	default:
		// Legacy mode - if the argument is a yaml file, run TUI
		if len(os.Args) == 2 && (endsWith(os.Args[1], ".yaml") || endsWith(os.Args[1], ".yml")) {
			runTUI(os.Args[1])
		} else {
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Println("API-Man - Filesystem-based API request management tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  api-man init                           Initialize workspace with default configs")
	fmt.Println("  api-man generate <spec.yaml>           Generate request configs from OpenAPI spec")
	fmt.Println("  api-man run <request> <env>            Execute a request with an environment")
	fmt.Println("  api-man list                           List all available requests")
	fmt.Println("  api-man envs                           List all available environments")
	fmt.Println("  api-man tui <spec.yaml>                Run TUI mode with OpenAPI spec")
	fmt.Println("  api-man body <command> [args]          Manage JSON body templates")
	fmt.Println()
	fmt.Println("Body commands:")
	fmt.Println("  api-man body list <request>            List all body JSON files for a request")
	fmt.Println("  api-man body set <request> <name>      Set active body JSON file")
	fmt.Println("  api-man body remove <request> <name>   Remove a body JSON file")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  api-man init")
	fmt.Println("  api-man generate openapi.yaml")
	fmt.Println("  api-man run users/get-users dev")
	fmt.Println("  api-man body list users/post-user")
	fmt.Println("  api-man body set users/post-user admin")
}

func initializeWorkspace() {
	cm, err := NewConfigManager()
	if err != nil {
		log.Fatal("Error initializing workspace:", err)
	}
	
	fmt.Println("✓ Initialized API-Man workspace")
	fmt.Printf("✓ Created directories: %s\n", cm.configDir)
	fmt.Println("✓ Generated default environments (dev, prod)")
	fmt.Println("✓ Created sample request")
	fmt.Println()
	fmt.Printf("Configuration directory: %s\n", cm.configDir)
	fmt.Println("You can now:")
	fmt.Println("  - Edit environment files in environments/")
	fmt.Println("  - Create request files in requests/")
	fmt.Println("  - Run: api-man list")
}

func generateFromOpenAPI(specFile string) {
	spec, err := LoadOpenAPISpec(specFile)
	if err != nil {
		log.Fatal("Error loading OpenAPI spec:", err)
	}

	cm, err := NewConfigManager()
	if err != nil {
		log.Fatal("Error initializing config manager:", err)
	}

	err = cm.GenerateRequestsFromOpenAPI(spec)
	if err != nil {
		log.Fatal("Error generating requests:", err)
	}

	fmt.Printf("✓ Generated request configurations from %s\n", specFile)
	fmt.Println("✓ Requests saved to ~/.api-man/requests/")
	fmt.Println()
	fmt.Println("Run 'api-man list' to see all generated requests")
}

func runRequest(requestPath, envName string) {
	cm, err := NewConfigManager()
	if err != nil {
		log.Fatal("Error initializing config manager:", err)
	}

	resp, err := cm.ExecuteRequest(requestPath, envName)
	if err != nil {
		log.Fatal("Error executing request:", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Printf("Headers:\n")
	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}
	fmt.Printf("\nResponse Body:\n")
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading response body:", err)
	}

	// Try to pretty print JSON
	var jsonObj interface{}
	if err := json.Unmarshal(body, &jsonObj); err == nil {
		prettyJSON, err := json.MarshalIndent(jsonObj, "", "  ")
		if err == nil {
			fmt.Println(string(prettyJSON))
		} else {
			fmt.Println(string(body))
		}
	} else {
		fmt.Println(string(body))
	}
}

func listRequests() {
	cm, err := NewConfigManager()
	if err != nil {
		log.Fatal("Error initializing config manager:", err)
	}

	requests, err := cm.ListRequests()
	if err != nil {
		log.Fatal("Error listing requests:", err)
	}

	fmt.Println("Available requests:")
	fmt.Println()
	for dir, reqList := range requests {
		fmt.Printf("📁 %s/\n", dir)
		for _, req := range reqList {
			config, err := cm.LoadRequest(req)
			if err != nil {
				continue
			}
			fmt.Printf("  🌐 %s - %s %s\n", req, config.Method, config.URL)
			if config.Description != "" {
				fmt.Printf("     %s\n", config.Description)
			}
		}
		fmt.Println()
	}
}

func listEnvironments() {
	cm, err := NewConfigManager()
	if err != nil {
		log.Fatal("Error initializing config manager:", err)
	}

	environments, err := cm.ListEnvironments()
	if err != nil {
		log.Fatal("Error listing environments:", err)
	}

	fmt.Println("Available environments:")
	fmt.Println()
	for _, env := range environments {
		envConfig, err := cm.LoadEnvironment(env)
		if err != nil {
			fmt.Printf("  ❌ %s (error loading)\n", env)
			continue
		}
		fmt.Printf("  🌍 %s - %s\n", env, envConfig.BaseURL)
	}
}

func runTUI(specFile string) {
	spec, err := LoadOpenAPISpec(specFile)
	if err != nil {
		log.Fatal("Error loading OpenAPI spec:", err)
	}

	// Initialize the TUI model
	m := NewModel(spec)

	// Start the TUI
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func handleBodyCommand() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: api-man body <command> [args]")
		fmt.Println("Commands: list, set, remove")
		os.Exit(1)
	}

	cm, err := NewConfigManager()
	if err != nil {
		log.Fatal("Error initializing config manager:", err)
	}

	subCommand := os.Args[2]

	switch subCommand {
	case "list":
		if len(os.Args) < 4 {
			fmt.Println("Usage: api-man body list <request-path>")
			os.Exit(1)
		}
		listBodies(cm, os.Args[3])
	case "set":
		if len(os.Args) < 5 {
			fmt.Println("Usage: api-man body set <request-path> <body-name>")
			os.Exit(1)
		}
		setActiveBody(cm, os.Args[3], os.Args[4])
	case "remove":
		if len(os.Args) < 5 {
			fmt.Println("Usage: api-man body remove <request-path> <body-name>")
			os.Exit(1)
		}
		removeBody(cm, os.Args[3], os.Args[4])
	default:
		fmt.Printf("Unknown body command: %s\n", subCommand)
		fmt.Println("Available commands: list, set, remove")
		os.Exit(1)
	}
}

func listBodies(cm *ConfigManager, requestPath string) {
	bodyFiles, activeBody, err := cm.ListBodies(requestPath)
	if err != nil {
		log.Fatal("Error listing bodies:", err)
	}

	fmt.Printf("Body JSON files for %s:\n\n", requestPath)
	
	if len(bodyFiles) == 0 {
		fmt.Println("No body JSON files found.")
		fmt.Println("You can create body files like 'admin.json', 'test.json', etc. in this directory.")
		return
	}

	for _, name := range bodyFiles {
		marker := " "
		if name == activeBody {
			marker = "●"
		}
		fmt.Printf("%s %s.json\n", marker, name)
		
		// Show first line of content as preview
		requestDir := filepath.Join("requests", requestPath)
		bodyFilePath := filepath.Join(requestDir, name+".json")
		if content, err := os.ReadFile(bodyFilePath); err == nil {
			lines := strings.Split(strings.TrimSpace(string(content)), "\n")
			if len(lines) > 0 {
				preview := lines[0]
				if len(preview) > 80 {
					preview = preview[:77] + "..."
				}
				fmt.Printf("  %s\n", preview)
			}
		}
		fmt.Println()
	}
	
	if activeBody != "" {
		fmt.Printf("Active body file: %s.json\n", activeBody)
	} else {
		fmt.Printf("Using default body from request.json\n")
	}
}


func setActiveBody(cm *ConfigManager, requestPath, bodyName string) {
	err := cm.SetActiveBody(requestPath, bodyName)
	if err != nil {
		log.Fatal("Error setting active body:", err)
	}

	fmt.Printf("✓ Set '%s' as active body template for %s\n", bodyName, requestPath)
}

func removeBody(cm *ConfigManager, requestPath, bodyName string) {
	err := cm.RemoveBody(requestPath, bodyName)
	if err != nil {
		log.Fatal("Error removing body:", err)
	}

	fmt.Printf("✓ Removed body template '%s' from %s\n", bodyName, requestPath)
}
