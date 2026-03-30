// Package bootstrap provides shared dependency construction logic
// for all abot binaries (agent, server, web).
//
// This package extracts common code from cmd/abot/main.go to avoid
// duplication across multiple binaries.
//
// # Key Functions
//
// Configuration:
//   - LoadConfig: Load configuration from YAML file
//   - LoadConfigWithEnv: Load config with environment variable expansion (future)
//
// Validation:
//   - ValidateForAgent: Validate config for agent mode (minimal requirements)
//   - ValidateForServer: Validate config for server mode (requires MySQL)
//   - ValidateForWeb: Validate config for web mode (requires MySQL + jwt_secret)
//
// Dependency Construction:
//   - BuildCoreDeps: Build minimal dependencies without MySQL (for agent mode)
//   - BuildFullDeps: Build complete dependencies with MySQL (for server/web mode)
//
// Component Builders:
//   - NewDatabase: Connect to MySQL and run migrations
//   - NewStores: Create all MySQL-backed stores
//   - NewSessionService: Create session service (jsonl or in-memory)
//   - NewProviders: Create LLM providers with fallback support
//   - NewChannels: Create channel adapters (WeCom, Telegram, Discord, Feishu)
//   - BuildLightweightTools: Build tools without database dependencies
//   - BuildFullTools: Build all tools including database-dependent ones
//
// # Usage Examples
//
// Agent mode (minimal dependencies, no MySQL):
//
//	cfg, err := bootstrap.LoadConfig("config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if err := bootstrap.ValidateForAgent(cfg); err != nil {
//	    log.Fatal(err)
//	}
//	deps, err := bootstrap.BuildCoreDeps(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	app, err := agent.Bootstrap(ctx, *cfg, *deps)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	app.Run(ctx)
//
// Server mode (full dependencies with MySQL):
//
//	cfg, err := bootstrap.LoadConfig("config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if err := bootstrap.ValidateForServer(cfg); err != nil {
//	    log.Fatal(err)
//	}
//	result, err := bootstrap.BuildFullDeps(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	app, err := agent.Bootstrap(ctx, *cfg, *result.Deps)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	app.Run(ctx)
//
// Web mode (full dependencies with MySQL and web console):
//
//	cfg, err := bootstrap.LoadConfig("config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	if err := bootstrap.ValidateForWeb(cfg); err != nil {
//	    log.Fatal(err)
//	}
//	result, err := bootstrap.BuildFullDeps(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	app, err := agent.Bootstrap(ctx, *cfg, *result.Deps)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	app.Run(ctx)
//
// # Architecture
//
// The bootstrap package follows a layered architecture:
//
//  1. Configuration Layer: LoadConfig, validation functions
//  2. Component Layer: Individual component builders (database, session, providers, etc.)
//  3. Integration Layer: BuildCoreDeps and BuildFullDeps that compose all components
//
// BuildCoreDeps includes:
//   - Session service (jsonl/memory)
//   - LLM providers with fallback
//   - Message bus
//   - Lightweight tools (no database required)
//   - MCP tools (optional)
//
// BuildFullDeps includes everything in BuildCoreDeps plus:
//   - MySQL database and all stores
//   - Full tools (including database-dependent ones)
//   - Channels (WeCom, Telegram, Discord, Feishu)
//   - Skills loader and builtin skills
//   - Vector store and embedder (optional)
//   - Memory consolidation plugin (optional)
//   - Cron scheduler
//   - Heartbeat service (optional)
//   - Marketplace API handler
//
// # Error Handling
//
// All functions return errors wrapped with context using fmt.Errorf with %w.
// This allows callers to use errors.Is and errors.As for error inspection.
//
// # Logging
//
// The package uses log/slog for structured logging. Key operations are logged
// at Info level, errors at Error level, and warnings at Warn level.
package bootstrap
