// Spawnbot - Personal AI assistant
// License: MIT
//
// Copyright (c) 2026 Spawnbot contributors

package config

// Runtime environment variable keys for the spawnbot process.
// These control the location of files and binaries at runtime and are read
// directly via os.Getenv / os.LookupEnv. All spawnbot-specific keys use the
// SPAWNBOT_ prefix. Reference these constants instead of inline string
// literals to keep all supported knobs visible in one place and to prevent
// typos.
const (
	// EnvHome overrides the base directory for all spawnbot data
	// (config, workspace, skills, auth store, …).
	// Default: ~/.spawnbot
	EnvHome = "SPAWNBOT_HOME"

	// EnvConfig overrides the full path to the JSON config file.
	// Default: $SPAWNBOT_HOME/config.json
	EnvConfig = "SPAWNBOT_CONFIG"

	// EnvBuiltinSkills overrides the directory from which built-in
	// skills are loaded.
	// Default: <cwd>/skills
	EnvBuiltinSkills = "SPAWNBOT_BUILTIN_SKILLS"

	// EnvBinary overrides the path to the spawnbot executable.
	// Used by the web launcher when spawning the gateway subprocess.
	// Default: resolved from the same directory as the current executable.
	EnvBinary = "SPAWNBOT_BINARY"

	// EnvGatewayHost overrides the host address for the gateway server.
	// Default: "127.0.0.1"
	EnvGatewayHost = "SPAWNBOT_GATEWAY_HOST"
)
