// Package main is the entry point for the TUI coding agent manager (codeg).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"codeg/internal/acp"
	"codeg/internal/agent"
	"codeg/internal/agent/core"
	opencodeagent "codeg/internal/agent/opencode"
	"codeg/internal/config"
	"codeg/internal/llm"
	"codeg/internal/session"
	"codeg/internal/task"
	"codeg/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	configPath := flag.String("config", "", "Path to config file (default: ~/.codeg/config.json)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	if *debug {
		f, err := tea.LogToFile("codeg-debug.log", "codeg")
		if err != nil {
			log.Fatalf("failed to open debug log: %v", err)
		}
		defer f.Close()
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := cfg.EnsureDir(); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Initialize session store
	sessionCfg := session.SQLiteConfig{
		Path:      cfg.DBPath,
		EnableWAL: true,
	}
	sessionStore, err := session.NewSQLiteStore(ctx, sessionCfg)
	if err != nil {
		log.Fatalf("Failed to create session store: %v", err)
	}
	defer sessionStore.Close()

	// Initialize task store (shares DB with session store — closed via sessionStore.Close)
	taskStore, err := task.NewSQLiteTaskStore(ctx, cfg.DBPath, sessionStore.GetDB())
	if err != nil {
		log.Fatalf("Failed to create task store: %v", err)
	}

	// Initialize sub-task store (shares the same DB)
	subTaskStore, err := task.NewSQLiteSubTaskStore(sessionStore.GetDB())
	if err != nil {
		log.Fatalf("Failed to create sub-task store: %v", err)
	}

	// Initialize LLM client
	var llmClient llm.Client
	if cfg.OpenAI.APIKey != "" {
		llmClient = llm.NewOpenAIClient(llm.OpenAIConfig{
			APIKey:  cfg.OpenAI.APIKey,
			BaseURL: cfg.OpenAI.BaseURL,
			Model:   cfg.OpenAI.Model,
		})
	}

	// Initialize task manager with sub-task support
	taskMgr := task.NewTaskManager(taskStore, llmClient)
	subTaskMgr := task.NewSubTaskManager(subTaskStore)
	taskMgr.SetSubTaskManager(subTaskMgr)

	// Initialize ACP agent runner.
	// If ACP server URL is configured, connect to an existing server;
	// otherwise, manage our own opencode serve process.
	var agentRunner agent.AgentRunner
	var acpMgr *agent.AcpManager

	if cfg.ACPServerURL != "" {
		// Connect to an external ACP server
		agentRunner = opencodeagent.NewRunner(cfg.ACPServerURL)
	} else {
		// Start and manage our own ACP server process
		acpMgr = agent.NewAcpManager(acp.ACPServerConfig{
			Command:  cfg.ACP.Command,
			PortFlag: cfg.ACP.PortFlag,
			Port:     cfg.ACP.Port,
			Provider: cfg.ACP.Provider,
			Model:    cfg.ACP.Model,
		})

		if err := acpMgr.StartServer(ctx); err != nil {
			log.Fatalf("Failed to start ACP server: %v", err)
		}
		defer acpMgr.StopServer()

		agentRunner = opencodeagent.NewRunner(acpMgr.ServerURL())
	}

	// Initialize agent controller
	agentCtrl := core.NewAgentController(agentRunner)

	// Create and run the TUI
	model := tui.NewModel(taskMgr, agentCtrl, taskStore)

	// Wire AcpManager into the model if available
	if acpMgr != nil {
		model.SetAcpManager(acpMgr)
	}

	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
