package rex

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/heroku/instruments/reporter"
	"github.com/juju/loggo"
	"github.com/mailgun/manners"
	"github.com/stvp/rollbar"
)

type Config struct {
	EventMetadata
	SentryDSN    string
	LogSpec      string
	KafkaBroker  string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
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
	config.ReadTimeout = 120 * time.Second
	config.WriteTimeout = 120 * time.Second
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
	Log            loggo.Logger
	Flags          flag.FlagSet
	Tracker        Tracker
	MetricsTicker  *MetricsTicker
	Listener       net.Listener
	Server         *manners.GracefulServer
	DebugServer    *manners.GracefulServer
	BaseConfig     *Config
	ReloadCallback func()
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

	// event metadata
	service.Flags.StringVar(&config.Environment, "environment", config.Environment, "Evironment to run in")
	service.Flags.StringVar(&config.Cluster, "cluster", config.Cluster, "Cluster to run in")

	// tracker options
	service.Flags.StringVar(&config.KafkaBroker, "kafka", config.KafkaBroker, "Initial Kafka Broker")

	// loggo options
	service.Flags.StringVar(&config.LogSpec, "loggo", config.LogSpec, "initial loggo spec")

	// sentry options
	service.Flags.StringVar(&rollbar.Token, "rollbar-token", rollbar.Token, "Sentry DSN")
}

func (service *Service) Run() {
	config := service.BaseConfig

	service.Log.Infof("command line arguments=%q", readArgs())
	service.Flags.Parse(readArgs())
	os.Setenv("REX_ENV", config.Environment)

	loggo.ConfigureLoggers(config.LogSpec)

	var err error
	service.Tracker, err = NewKafkaTracker(config.KafkaBroker, &config.EventMetadata)
	MayPanic(err)

	service.MetricsTicker = NewMetricsTicker(service.Tracker)
	go service.MetricsTicker.Start()

	if service.BaseConfig.Port > 0 {
		service.DebugServer = StartDebugServer(service.BaseConfig.Port + 9)

		service.Listener, err = net.Listen("tcp", fmt.Sprintf(":%d", service.BaseConfig.Port))
		MayPanic(err)
		service.Log.Infof("start listen=:%d", service.BaseConfig.Port)
	}
}

func (service *Service) Serve(handler http.Handler) {
	service.Server = manners.NewWithServer(&http.Server{
		Handler:      handler,
		ReadTimeout:  service.BaseConfig.ReadTimeout,
		WriteTimeout: service.BaseConfig.WriteTimeout,
		ConnState:    service.ConnStateHandler,
	})

	service.Log.Infof("now serving requests on listener")
	MayPanic(service.Server.Serve(service.Listener))
	service.Log.Infof("stopped serving on listener")
}

var newConnections = reporter.NewRegisteredRate("listener.connections.new")
var activeConnections = reporter.NewRegisteredRate("listener.connections.active")
var closedConnections = reporter.NewRegisteredRate("listener.connections.closed")

func (service *Service) ConnStateHandler(conn net.Conn, state http.ConnState) {
	switch state {
	case http.StateNew:
		newConnections.Update(1)
	case http.StateActive:
		activeConnections.Update(1)
	case http.StateClosed:
		closedConnections.Update(1)
	}
}

func (service *Service) CloseWait() {
	if service.Server != nil {
		service.Log.Infof("shutting down http server")
		service.Server.Close()
		service.Listener = nil
		service.Log.Infof("waiting for requests to finish")
		service.Server.Wait()
		service.Server = nil
		service.Log.Infof("all request handlers done")
	}
	if service.Listener != nil {
		service.Log.Infof("closing dangling listener")
		CaptureError(service.Listener.Close())
	}
	if service.DebugServer != nil {
		service.Log.Infof("shutting down debug server")
		service.DebugServer.Close()
		service.Log.Infof("waiting for requests to finish")
		service.DebugServer.Wait()
		service.DebugServer = nil
		service.Log.Infof("all request handlers done")
	}
}

func (service *Service) Shutdown() {
	service.Log.Infof("service shutdown")
	service.CloseWait()
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
	signal.Notify(
		ch,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
	)
	for {
		sig := <-ch
		service.Log.Infof("caught signal %s. shutting down", sig.String())
		go func() {
			time.Sleep(5 * time.Minute)
			service.Log.Infof("still not dead. trying to exit")
			pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
			go func() {
				time.Sleep(30 * time.Second)
				service.Log.Infof("still not dead. killing myself")
				pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
				syscall.Kill(os.Getpid(), syscall.SIGKILL)
			}()
			os.Exit(-1)
		}()
		shutdownCallback()
		return sig.(syscall.Signal), nil
	}
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
