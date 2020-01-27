package cmd

import (
	"io/ioutil"
	golog "log"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/ubccr/grendel/api"
	"github.com/ubccr/grendel/logger"
	"github.com/ubccr/grendel/util"
)

var (
	cfgFile     string
	cfgFileUsed string
	debug       bool
	verbose     bool

	Log  = logger.GetLogger("CLI")
	Root = &cobra.Command{
		Use:     "grendel",
		Version: api.Version,
		Short:   "Provisioning system for high-performance Linux clusters",
		Long:    ``,
	}
)

func Execute() {
	if err := Root.Execute(); err != nil {
		Log.Fatal(err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	Root.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file")
	Root.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug messages")
	Root.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose messages")

	Root.PersistentPreRunE = func(command *cobra.Command, args []string) error {
		return SetupLogging()
	}
}

func SetupLogging() error {
	if debug {
		Log.Logger.SetLevel(logrus.DebugLevel)
	} else if verbose {
		Log.Logger.SetLevel(logrus.InfoLevel)
	} else {
		Log.Logger.SetLevel(logrus.WarnLevel)
	}
	golog.SetOutput(ioutil.Discard)

	if cfgFileUsed != "" {
		Log.Infof("Using config file: %s", cfgFileUsed)
	}

	Root.SilenceUsage = true
	Root.SilenceErrors = true

	return nil
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			Log.Fatal(err)
		}

		viper.AddConfigPath("/etc/grendel/")
		viper.AddConfigPath(home)
		viper.SetConfigName(".grendel")
		viper.SetConfigType("toml")
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("grendel")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := viper.ReadInConfig(); err == nil {
		cfgFileUsed = viper.ConfigFileUsed()
	}

	if !viper.IsSet("provision.secret") {
		secret, err := util.GenerateSecret(32)
		if err != nil {
			Log.Fatal(err)
		}

		viper.Set("provision.secret", secret)
	}

	if !viper.IsSet("api.secret") {
		secret, err := util.GenerateSecret(32)
		if err != nil {
			Log.Fatal(err)
		}

		viper.Set("api.secret", secret)
	}
}