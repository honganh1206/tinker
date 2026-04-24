package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/honganh1206/tinker/mcp"
	"github.com/spf13/cobra"
)

var (
	verbose          bool
	mcpServerCmd     string
	mcpServerConfigs []mcp.ServerConfig
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func MCPHandler(cmd *cobra.Command, args []string) error {
	if mcpServerCmd != "" {
		parts := strings.SplitN(mcpServerCmd, ":", 2)
		if len(parts) == 2 {
			id := strings.TrimSpace(parts[0])
			command := strings.TrimSpace(parts[1])
			if id != "" && command != "" {
				config := mcp.ServerConfig{
					ID:      id,
					Command: command,
				}
				mcpServerConfigs = append(mcpServerConfigs, config)
				if verbose {
					fmt.Printf("Added server configuration from flag: %s -> %s\n", id, command)
				}
			} else {
				return fmt.Errorf("invalid server configuration format in flag: %s (expected id:command)", mcpServerCmd)
			}
		} else {
			return fmt.Errorf("invalid server configuration format in flag: %s (expected id:command)", mcpServerCmd)
		}
	}

	if len(mcpServerConfigs) == 0 {
		return errors.New("no server configurations provided (use --server-cmd flag or provide id:command arguments)")
	}

	if err := mcp.SaveConfigs(mcpServerConfigs); err != nil {
		if verbose {
			fmt.Printf("Warning: Could not save configurations: %v\n", err)
		}
	} else if verbose {
		fmt.Printf("Saved %d server configurations to file\n", len(mcpServerConfigs))
	}

	if verbose {
		fmt.Printf("Total server configurations: %d\n", len(mcpServerConfigs))
		for _, config := range mcpServerConfigs {
			fmt.Printf("  - %s: %s\n", config.ID, config.Command)
		}
	}

	return nil
}

func NewCLI() *cobra.Command {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number of tinker",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Tinker version %s (commit: %s, built: %s)\n", Version, GitCommit, BuildTime)
		},
	}

	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server",
		Long: `Start an MCP (Model Context Protocol) server with the specified configuration.

Server configurations must be in the format id:command.

Examples:
  tinker mcp --server-cmd "my-server:uvx mcp-server-fetch"
  tinker mcp "fetch-server:uvx mcp-server-fetch"
  tinker mcp "python-server:python my_mcp_server.py --port 8080"
  tinker mcp --verbose "node-server:node mcp-server.js"
  tinker mcp "server1:uvx mcp-server-fetch" "server2:python other_server.py"`,
		RunE: MCPHandler,
	}

	mcpCmd.Flags().StringVar(&mcpServerCmd, "server-cmd", "", "Server configuration in format id:command (e.g., 'my-server:uvx mcp-server-fetch')")

	rootCmd := &cobra.Command{
		Use:   "tinker",
		Short: "A background coding agent",
		Long:  `Tinker is a background coding agent. Agent runs are triggered via channel messages (e.g., Discord).`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if configs, err := mcp.LoadConfigs(); err == nil {
				mcpServerConfigs = configs
				if verbose && len(configs) > 0 {
					fmt.Printf("Loaded %d MCP server configurations\n", len(configs))
				}
			}
		},
	}

	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")

	rootCmd.AddCommand(versionCmd, mcpCmd)

	return rootCmd
}
