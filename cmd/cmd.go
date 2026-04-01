package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/honganh1206/tinker/inference"
	"github.com/honganh1206/tinker/mcp"
	"github.com/honganh1206/tinker/session"
	"github.com/honganh1206/tinker/store"
	"github.com/spf13/cobra"
)

var (
	llm              inference.BaseLLMClient
	verbose          bool
	mcpServerCmd     string
	mcpServerConfigs []mcp.ServerConfig
)

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func RunHandler(cmd *cobra.Command, args []string) error {
	prompt, _ := cmd.Flags().GetString("prompt")
	if prompt == "" && len(args) > 0 {
		prompt = strings.Join(args, " ")
	}
	if prompt == "" {
		return fmt.Errorf("prompt is required: tinker \"your prompt here\"")
	}

	verifyCmd, _ := cmd.Flags().GetString("verify-cmd")

	provider := inference.ProviderName(llm.Provider)
	if llm.Model == "" {
		llm.Model = string(inference.GetDefaultModel(provider))
	}
	if llm.TokenLimit == 0 {
		llm.TokenLimit = 8192
	}

	cfg := session.SessionConfig{
		LLMBase:    llm,
		MCPConfigs: mcpServerConfigs,
		Prompt:     prompt,
		VerifyCmd:  verifyCmd,
		Verbose:    verbose,
	}

	result, err := session.RunSession(cmd.Context(), cfg)
	if err != nil {
		return err
	}

	// Save session to file store
	s, err := store.NewFileStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save session: %v\n", err)
	} else if err := s.Save(result); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save session: %v\n", err)
	}

	return session.OutputResult(result)
}

func ModelHandler(cmd *cobra.Command, args []string) error {
	provider := inference.ProviderName(llm.Provider)
	models := inference.ListAvailableModels(provider)

	if len(models) > 0 {
		fmt.Printf("Available models for %s:\n", provider)
		for _, model := range models {
			fmt.Printf("  - %s\n", model)
		}
	} else {
		fmt.Printf("For %s, specify your custom model name with the --model flag\n", provider)
	}

	return nil
}

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

func SessionsHandler(cmd *cobra.Command, args []string) error {
	s, err := store.NewFileStore("")
	if err != nil {
		return fmt.Errorf("failed to open session store: %w", err)
	}

	summaries, err := s.List()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(summaries) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	for _, s := range summaries {
		fmt.Printf("%-10s %-8s %s\n", s.ID[:8], s.Status, s.Prompt)
	}

	return nil
}

func NewCLI() *cobra.Command {
	modelCmd := &cobra.Command{
		Use:   "model",
		Short: "List available models for the selected provider",
		RunE:  ModelHandler,
	}

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

	sessionsCmd := &cobra.Command{
		Use:   "sessions",
		Short: "List past sessions",
		RunE:  SessionsHandler,
	}

	rootCmd := &cobra.Command{
		Use:   "tinker",
		Short: "A background coding agent",
		Long: `Tinker is a background coding agent that runs headlessly.

Usage:
  tinker "fix the linting errors"
  tinker --verify-cmd "go test ./..." "add unit tests for auth"
  tinker --provider anthropic --model claude-4-sonnet "refactor the handler"

Output is structured JSON on stdout. Logs go to stderr.`,
		Args: cobra.ArbitraryArgs,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if configs, err := mcp.LoadConfigs(); err == nil {
				mcpServerConfigs = configs
				if verbose && len(configs) > 0 {
					fmt.Printf("Loaded %d MCP server configurations\n", len(configs))
				}
			}
		},
		RunE: RunHandler,
	}

	rootCmd.PersistentFlags().StringVar(&llm.Provider, "provider", string(inference.GoogleProvider), "Provider (anthropic, gemini)")
	rootCmd.PersistentFlags().StringVar(&llm.Model, "model", "", "Model to use (depends on selected model)")
	rootCmd.PersistentFlags().Int64Var(&llm.TokenLimit, "max-tokens", 0, "Maximum number of tokens in response")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")
	rootCmd.Flags().StringP("prompt", "p", "", "The task prompt")
	rootCmd.Flags().String("verify-cmd", "", "Verification command to run after agent completes")

	rootCmd.AddCommand(versionCmd, modelCmd, mcpCmd, sessionsCmd)

	return rootCmd
}
