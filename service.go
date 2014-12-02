package rex

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/getsentry/raven-go"
	"github.com/juju/loggo"
)

type Config struct {
	EventMetadata
	SentryDSN   string
	LogSpec     string
	KafkaBroker string
	Port        int
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

type Service struct {
	Log        loggo.Logger
	Flags      flag.FlagSet
	Tracker    Tracker
	BaseConfig *Config
}

func (service *Service) Init() {
	config := service.BaseConfig

	loggo.ReplaceDefaultWriter(loggo.NewSimpleWriter(os.Stdout, &LogFormat{}))
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

	// event metadata
	service.Flags.StringVar(&config.Environment, "environment", config.Environment, "Evironment to run in")
	service.Flags.StringVar(&config.Cluster, "cluster", config.Cluster, "Cluster to run in")

	// tracker options
	service.Flags.StringVar(&config.KafkaBroker, "kafka", config.KafkaBroker, "Initial Kafka Broker")

	// loggo options
	service.Flags.StringVar(&config.LogSpec, "loggo", config.LogSpec, "initial loggo spec")

	// sentry options
	service.Flags.StringVar(&config.SentryDSN, "sentry-dsn", config.SentryDSN, "Sentry DSN")
}

func (service *Service) Run() {
	service.Log.Infof("command line arguments=%q", readArgs())
	service.Flags.Parse(readArgs())

	var err error
	Raven, err = raven.NewClient(service.BaseConfig.SentryDSN, nil)
	MayPanic(err)

	loggo.ConfigureLoggers(service.BaseConfig.LogSpec)

	service.Tracker = NewKafkaTracker(service.BaseConfig)
	go TrackMetrics(service.Tracker)

	if service.BaseConfig.Port > 0 {
		ds := DebugServer{Port: service.BaseConfig.Port + 9}
		go ds.Start()
	}
}

func (service *Service) Wait() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch // will trigger defer from main()
}

func (service *Service) Shutdown() {
	service.Tracker.Close()
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
	Full bool
}

func (*LogFormat) Format(level loggo.Level, module, filename string, line int, timestamp time.Time, message string) string {
	// Just get the basename from the filename
	filename = filepath.Base(filename)
	return fmt.Sprintf("[%s] %s (at %s:%d)", module, message, filename, line)
}
