package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"xorkevin.dev/klog"
)

type (
	Cmd struct {
		rootCmd    *cobra.Command
		log        *klog.LevelLogger
		version    string
		rootFlags  rootFlags
		modelFlags modelFlags
		validFlags validFlags
		docFlags   docFlags
	}

	rootFlags struct {
		logLevel string
		logJSON  bool
	}
)

func New() *Cmd {
	return &Cmd{}
}

func (c *Cmd) Execute() {
	buildinfo := ReadVCSBuildInfo()
	c.version = buildinfo.ModVersion
	if overrideVersion := os.Getenv("FORGE_OVERRIDE_VERSION"); overrideVersion != "" {
		c.version = overrideVersion
	}
	rootCmd := &cobra.Command{
		Use:   "forge",
		Short: "A code generation utility",
		Long: `A code generation utility for governor to generate common files instead
of writing them by hand.`,
		Version:           c.version,
		PersistentPreRun:  c.initConfig,
		DisableAutoGenTag: true,
	}
	rootCmd.PersistentFlags().StringVar(&c.rootFlags.logLevel, "log-level", "info", "log level")
	rootCmd.PersistentFlags().BoolVar(&c.rootFlags.logJSON, "log-json", false, "output json logs")
	c.rootCmd = rootCmd

	rootCmd.AddCommand(c.getModelCmd())
	rootCmd.AddCommand(c.getValidationCmd())
	rootCmd.AddCommand(c.getDocCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
		return
	}
}

// initConfig reads in config file and ENV variables if set.
func (c *Cmd) initConfig(cmd *cobra.Command, args []string) {
	logWriter := klog.NewSyncWriter(os.Stderr)
	var handler *klog.SlogHandler
	if c.rootFlags.logJSON {
		handler = klog.NewJSONSlogHandler(logWriter)
	} else {
		handler = klog.NewTextSlogHandler(logWriter)
		handler.FieldTime = ""
		handler.FieldCaller = ""
		handler.FieldMod = ""
	}
	c.log = klog.NewLevelLogger(klog.New(
		klog.OptHandler(handler),
		klog.OptMinLevelStr(c.rootFlags.logLevel),
	))
}

func (c *Cmd) logFatal(err error) {
	c.log.Err(context.Background(), err)
	os.Exit(1)
}
