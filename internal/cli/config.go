package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/daydemir/ralph/internal/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config [key] [value]",
	Short: "View or modify configuration",
	Long: `View or modify Ralph configuration.

Examples:
  ralph config                    Show all config
  ralph config llm.backend        Get a specific value
  ralph config llm.backend claude Set a value`,
	Args: cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsDir, err := workspace.Find()
		if err != nil {
			return err
		}

		configPath := filepath.Join(wsDir, ".ralph", "config.yaml")

		switch len(args) {
		case 0:
			return showConfig(configPath)
		case 1:
			return getConfigValue(configPath, args[0])
		case 2:
			return setConfigValue(configPath, args[0], args[1])
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func showConfig(configPath string) error {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}
	fmt.Println(string(content))
	return nil
}

func getConfigValue(configPath, key string) error {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	value := v.Get(key)
	if value == nil {
		return fmt.Errorf("key not found: %s", key)
	}

	fmt.Println(value)
	return nil
}

func setConfigValue(configPath, key, value string) error {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Handle array values (comma-separated)
	if strings.Contains(value, ",") {
		v.Set(key, strings.Split(value, ","))
	} else {
		v.Set(key, value)
	}

	if err := v.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}
