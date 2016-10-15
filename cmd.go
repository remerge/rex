package rex

import (
	"fmt"
	"os"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/remerge/rex/env"
	. "github.com/remerge/rex/log"
	"github.com/remerge/rex/rollbar"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var RootCmd = &cobra.Command{
	Use:   "rex",
	Short: "rex: remerge extensions",

	Run: func(cmd *cobra.Command, args []string) {
		viper.SetDefault("port", 9990)
		service := &Service{Name: "rex"}
		service.Init()
		go service.Run()
		service.Wait(service.Shutdown)
	},
}

var VersionCmd = &cobra.Command{
	Use:   "version",
	Short: "display version and exit",

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(CodeVersion)
	},
}

var cfgFile string
var envName string
var logSpec string

func Execute(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yml)")
	cmd.PersistentFlags().StringVar(&envName, "environment", "development", "environment to run in (development, test, production)")
	cmd.PersistentFlags().StringVar(&logSpec, "log", "<root>=INFO", "logger configuration")
	cmd.PersistentFlags().StringVar(&rollbar.Token, "rollbar", "", "rollbar token")

	cmd.AddCommand(VersionCmd)
	cobra.OnInitialize(initConfig)

	if err := cmd.Execute(); err != nil {
		os.Exit(-1)
	}
}

func initConfig() {
	// setup env
	env.Set(envName)

	// load config file
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}

	viper.SetConfigName(envName)
	viper.AddConfigPath("./config")
	viper.AutomaticEnv()
	viper.ReadInConfig()

	// configure logger
	ConfigureLoggers(logSpec)

	// configure rollbar
	rollbar.Environment = envName
	rollbar.CodeVersion = CodeVersion

	// use all cores by default
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	// gin mode
	switch envName {
	case "production":
		gin.SetMode("release")
	case "test", "testing":
		gin.SetMode("test")
	default:
		gin.SetMode("debug")
	}
}
