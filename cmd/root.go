package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	versionString = "v0.3"
)

var (
	cfgFile   string
	debugMode bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "forge",
	Short: "A code generation utility",
	Long: `A code generation utility for governor to generate common files instead
of writing them by hand.`,
	Version: versionString,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $XDG_CONFIG_HOME/.forge.yaml)")
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "turn on debug output")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName(".forge")
		viper.AddConfigPath(".")

		// Search config in XDG_CONFIG_HOME directory with name ".forge" (without extension).
		cfgdir, err := os.UserConfigDir()
		if err == nil {
			viper.AddConfigPath(cfgdir)
		}
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	configErr := viper.ReadInConfig()
	if debugMode {
		if configErr == nil {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		} else {
			fmt.Fprintln(os.Stderr, "Failed reading config file:", configErr)
		}
	}
}
