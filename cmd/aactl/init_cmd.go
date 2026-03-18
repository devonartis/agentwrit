package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/divineartis/agentauth/internal/cfg"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

var (
	initMode       string
	initForce      bool
	initConfigPath string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate admin secret and write config file",
	Long: `Initialize AgentAuth by generating a cryptographically secure admin
secret and writing a config file. In dev mode, the plaintext is stored
in the config for easy retrieval. In prod mode, only the bcrypt hash
is stored -- the plaintext is shown once and never saved to disk.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		secret, err := runInit(initMode, resolveConfigPath(), initForce)
		if err != nil {
			return err
		}
		fmt.Println()
		fmt.Printf("Admin secret: %s\n", secret)
		fmt.Println()
		if initMode == "prod" {
			fmt.Println("WARNING: Save this secret now. It will not be shown again.")
			fmt.Println("Store it in your secrets manager (Vault, AWS Secrets Manager, etc.).")
		} else {
			fmt.Println("Dev mode: secret is also stored in the config file.")
		}
		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&initMode, "mode", "dev", "initialization mode: dev or prod")
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite existing config file")
	initCmd.Flags().StringVar(&initConfigPath, "config-path", "", "explicit config file path")
	rootCmd.AddCommand(initCmd)
}

// resolveConfigPath returns the config file path based on flags and defaults.
func resolveConfigPath() string {
	if initConfigPath != "" {
		return initConfigPath
	}
	if p := os.Getenv("AA_CONFIG_PATH"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/etc/agentauth/config"
	}
	return home + "/.agentauth/config"
}

// runInit generates a secret and writes the config file. Returns the
// plaintext secret.
func runInit(mode, configPath string, force bool) (string, error) {
	if mode != "dev" && mode != "prod" {
		return "", fmt.Errorf("invalid mode %q: must be 'dev' or 'prod'", mode)
	}

	if force {
		fmt.Fprintf(os.Stderr, "WARNING: Overwriting existing config at %s\n", configPath)
	}

	// Generate 32-byte cryptographically random secret.
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)

	// Determine what to write to config.
	var configMode, configValue string
	if mode == "prod" {
		configMode = "production"
		hash, err := bcrypt.GenerateFromPassword([]byte(secret), 12)
		if err != nil {
			return "", fmt.Errorf("hash secret: %w", err)
		}
		configValue = string(hash)
	} else {
		configMode = "development"
		configValue = secret
	}

	if err := cfg.WriteConfigFile(configPath, configMode, configValue, force); err != nil {
		return "", err
	}

	fmt.Printf("Config written to: %s\n", configPath)
	return secret, nil
}
