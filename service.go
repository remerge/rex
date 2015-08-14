package rex

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juju/loggo"
	"github.com/remerge/rex/rollbar"
)

type Config struct {
	EventMetadata
	LogSpec     string
	KafkaBroker string
	Port        int
	TlsPort     int
	TlsKey      string
	TlsCert     string
}

func NewConfig(name string, port int) *Config {
	config := &Config{}
	config.Service = name
	config.Environment = "development"
	config.Cluster = "development"
	config.Host = GetFQDN()
	config.LogSpec = "<root>=INFO"
	config.KafkaBroker = "0.0.0.0:9092"
	config.Port = port
	return config
}

const (
	SIGHUP  = syscall.SIGHUP
	SIGINT  = syscall.SIGINT
	SIGQUIT = syscall.SIGQUIT
	SIGTERM = syscall.SIGTERM
	SIGUSR1 = syscall.SIGUSR1
	SIGUSR2 = syscall.SIGUSR2
)

type Service struct {
	Log           loggo.Logger
	Flags         flag.FlagSet
	Tracker       Tracker
	MetricsTicker *MetricsTicker
	DebugServer   *Listener
	BaseConfig    *Config
}

func (service *Service) Init() {
	config := service.BaseConfig

	loggo.ReplaceDefaultWriter(loggo.NewSimpleWriter(os.Stdout, &LogFormat{Service: config.Service}))
	rootLogger := loggo.GetLogger("")
	rootLogger.SetLogLevel(loggo.INFO)
	service.Log = loggo.GetLogger(config.Service)

	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	service.Log.Infof("using %d cores for go routines", runtime.GOMAXPROCS(0))

	service.Flags.Init(os.Args[0], flag.ExitOnError)

	// listen port for service
	service.Flags.IntVar(&config.Port, "port", config.Port, "listen port")

	// TLS options
	service.Flags.IntVar(&config.TlsPort, "tls-port", config.TlsPort, "listen port for TLS")
	service.Flags.StringVar(&config.TlsKey, "tls-key", config.TlsKey, "TLS certificate key")
	service.Flags.StringVar(&config.TlsCert, "tls-cert", config.TlsCert, "TLS certificate")

	// event metadata
	service.Flags.StringVar(&config.Environment, "environment", config.Environment, "Evironment to run in")
	service.Flags.StringVar(&config.Cluster, "cluster", config.Cluster, "Cluster to run in")

	// tracker options
	service.Flags.StringVar(&config.KafkaBroker, "kafka", config.KafkaBroker, "Initial Kafka Broker")

	// loggo options
	service.Flags.StringVar(&config.LogSpec, "loggo", config.LogSpec, "initial loggo spec")

	// rollbar options
	service.Flags.StringVar(&rollbar.Token, "rollbar-token", rollbar.Token, "Rollbar API Token")
}

func (service *Service) ReadArgs() {
	service.Log.Infof("command line arguments=%q", readArgs())
	service.Flags.Parse(readArgs())
	os.Setenv("REX_ENV", service.BaseConfig.Environment)
	rollbar.Environment = service.BaseConfig.Environment
	rev, _ := exec.Command("git", "rev-parse", "HEAD").Output()
	rollbar.CodeVersion = string(bytes.TrimSpace(rev))
}

func (service *Service) Run() {
	service.ReadArgs()

	config := service.BaseConfig
	loggo.ConfigureLoggers(config.LogSpec)

	var err error
	service.Tracker, err = NewKafkaTracker(config.KafkaBroker, &config.EventMetadata)
	MayPanic(err)

	service.MetricsTicker = NewMetricsTicker(service.Tracker)
	go service.MetricsTicker.Start()

	if config.Environment == "production" {
		gin.SetMode("release")
	}

	if service.BaseConfig.Port > 0 {
		service.DebugServer = StartDebugServer(service.BaseConfig.Port + 9)
	}
}

func (service *Service) Shutdown() {
	service.Log.Infof("service shutdown")
	if service.DebugServer != nil {
		service.Log.Infof("shutting down debug server")
		service.DebugServer.Stop()
		service.Log.Infof("waiting for requests to finish")
		service.DebugServer.Wait()
		service.DebugServer = nil
	}
	service.Log.Infof("shutting down metrics ticker")
	service.MetricsTicker.Stop()
	service.Log.Infof("shutting down tracker")
	service.Tracker.Close()
	service.Log.Infof("waiting for rollbar")
	rollbar.Wait()
	service.Log.Infof("service shutdown done, dumping dangling go routines")
	pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
}

func (service *Service) Wait(shutdownCallback func()) (syscall.Signal, error) {
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT)
	for {
		sig := <-ch
		service.Log.Infof("caught signal %s. shutting down", sig.String())
		go service.shutdownCheck()
		shutdownCallback()
		return sig.(syscall.Signal), nil
	}
}

func (service *Service) shutdownCheck() {
	time.Sleep(1 * time.Minute)
	service.Log.Infof("still not dead. dumping dangling go routines")
	pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
	go service.shutdownCheck()
}

func readArgs() []string {
	var args []string

	contents, err := ioutil.ReadFile(".args")
	if err == nil {
		args = strings.Fields(string(contents))
	}

	// do not leak go test arguments into service
	if !strings.HasSuffix(os.Args[0], ".test") {
		args = append(args, os.Args[1:]...)
	}

	return args
}

type LogFormat struct {
	Service string
}

func (self *LogFormat) Format(level loggo.Level, module, filename string, line int, timestamp time.Time, message string) string {
	// Just get the basename from the filename
	filename = filepath.Base(filename)
	return fmt.Sprintf("%s[%d] [%s] %s (at %s:%d)", self.Service, os.Getpid(), module, message, filename, line)
}
