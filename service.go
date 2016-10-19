package rex

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	rp "runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/juju/loggo"
	"github.com/remerge/rex/env"
	"github.com/remerge/rex/log"
	"github.com/remerge/rex/rollbar"
	"github.com/tylerb/graceful"
)

type Config struct {
	EventMetadata
	LogSpec                 string
	KafkaBroker             string
	Port                    int
	TlsPort                 int
	TlsKey                  string
	TlsCert                 string
	ServerShutdownTimeout   time.Duration
	ServerConnectionTimeout time.Duration
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
	config.ServerShutdownTimeout = 30 * time.Second
	config.ServerConnectionTimeout = 2 * time.Minute
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
	BaseConfig    *Config
	CodeVersion   string
	CodeBuild     string
	Engine        *gin.Engine
	Server        *graceful.Server
	TlsServer     *graceful.Server
	DebugEngine   *gin.Engine
	DebugServer   *graceful.Server
	GinRecovery   gin.HandlerFunc
}

func (service *Service) InitLogger() {
	service.Log = log.GetLogger(service.BaseConfig.Service)
}

func (service *Service) InitCommandLine() {
	config := service.BaseConfig
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}
	service.Flags.Init(os.Args[0], flag.ExitOnError)
	service.Flags.StringVar(&config.LogSpec, "loggo", config.LogSpec, "initial loggo spec")
}

func (service *Service) InitDefaultFlags() {
	config := service.BaseConfig

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

	// rollbar options
	service.Flags.StringVar(&rollbar.Token, "rollbar-token", rollbar.Token, "Rollbar API Token")
}

func (service *Service) InitEngine() {
	if service.Engine == nil {
		service.Engine = gin.New()
		service.Engine.Use(
			rollbar.GinRecovery(),
			log.GinLogger(fmt.Sprintf("%s.engine", service.BaseConfig.Service)),
		)
	}

	if service.DebugEngine == nil {
		service.DebugEngine = gin.New()
		service.DebugEngine.Use(
			rollbar.GinRecovery(),
			log.GinLogger(fmt.Sprintf("%s.debug", service.BaseConfig.Service)),
		)
	}
}

func (service *Service) Init() {
	service.InitLogger()
	service.InitCommandLine()
	service.InitDefaultFlags()
	service.InitEngine()
}

func (service *Service) ReadArgs() {
	// set global version and build info
	service.CodeVersion = CodeVersion
	service.CodeBuild = CodeBuild

	// parse command line
	doVersion := service.Flags.Bool("version", false, "show version and exit")
	MayPanic(service.Flags.Parse(readArgs()))

	// show version and exit
	if *doVersion {
		fmt.Println(service.CodeVersion)
		os.Exit(0)
	}

	// set environment for children
	env.Set(service.BaseConfig.Environment)

	// setup rollbar
	rollbar.Environment = service.BaseConfig.Environment
	rollbar.CodeVersion = service.CodeVersion

	// add code version to event metadata
	service.BaseConfig.EventMetadata.Release = rollbar.CodeVersion

	// configure log levels
	MayPanic(log.ConfigureLoggers(service.BaseConfig.LogSpec))

	service.Log.Infof("code version=%v build=%v", service.CodeVersion, service.CodeBuild)
	service.Log.Infof("command line arguments=%q", readArgs())
	service.Log.Infof("using %d cores for go routines", runtime.GOMAXPROCS(0))
}

func (service *Service) Run() {
	if service.CodeVersion == "" {
		service.ReadArgs()
	}

	config := service.BaseConfig

	var err error
	service.Tracker, err = NewKafkaTracker(config.KafkaBroker, &config.EventMetadata)
	MayPanic(err)

	service.MetricsTicker = NewMetricsTicker(service.Tracker)
	go service.MetricsTicker.Start()

	if config.Environment == "production" {
		gin.SetMode("release")
	}

	if service.BaseConfig.Port > 0 {
		go service.ServeDebug()
	}
}

func (service *Service) Serve(handler http.Handler) {
	if handler == nil {
		handler = service.Engine
	}

	service.Server = &graceful.Server{
		Timeout:          service.BaseConfig.ServerShutdownTimeout,
		NoSignalHandling: true,
		Server: &http.Server{
			Handler: handler,
			Addr:    fmt.Sprintf(":%d", service.BaseConfig.Port),
		},
	}

	service.Server.ReadTimeout = service.BaseConfig.ServerConnectionTimeout
	service.Server.WriteTimeout = service.BaseConfig.ServerConnectionTimeout

	service.Log.Infof("start server listen %s", service.Server.Server.Addr)
	MayPanic(service.Server.ListenAndServe())
}

func (service *Service) ServeTLS(handler http.Handler) {
	if handler == nil {
		handler = service.Engine
	}

	service.TlsServer = &graceful.Server{
		Timeout: service.BaseConfig.ServerShutdownTimeout,
		Server: &http.Server{
			Handler: handler,
			Addr:    fmt.Sprintf(":%d", service.BaseConfig.TlsPort),
		},
		NoSignalHandling: true,
	}

	service.TlsServer.ReadTimeout = service.BaseConfig.ServerConnectionTimeout
	service.TlsServer.WriteTimeout = service.BaseConfig.ServerConnectionTimeout

	service.Log.Infof("start tls server listen %s", service.TlsServer.Server.Addr)
	MayPanic(service.TlsServer.ListenAndServeTLS(service.BaseConfig.TlsCert, service.BaseConfig.TlsKey))
}

func (service *Service) ServeDebug() {
	service.DebugEngine.GET("/loggo", log.GetLoggoSpec)
	service.DebugEngine.POST("/loggo", log.SetLoggoSpec)

	service.DebugEngine.GET("/debug/pprof/", gin.WrapF(pprof.Index))
	service.DebugEngine.GET("/debug/pprof/block", gin.WrapF(pprof.Index))
	service.DebugEngine.GET("/debug/pprof/cmdline", gin.WrapF(pprof.Cmdline))
	service.DebugEngine.GET("/debug/pprof/goroutine", gin.WrapF(pprof.Index))
	service.DebugEngine.GET("/debug/pprof/heap", gin.WrapF(pprof.Index))
	service.DebugEngine.GET("/debug/pprof/profile", gin.WrapF(pprof.Profile))
	service.DebugEngine.GET("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
	service.DebugEngine.POST("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
	service.DebugEngine.GET("/debug/pprof/threadcreate", gin.WrapF(pprof.Index))
	service.DebugEngine.GET("/debug/pprof/trace", gin.WrapF(pprof.Trace))

	service.DebugEngine.GET("/blockprof/:rate", func(c *gin.Context) {
		r, err := strconv.Atoi(c.Param("rate"))
		if err != nil {
			c.String(http.StatusOK, "rate invalid %s. %v", c.Param("rate"), err)
			return
		}
		runtime.SetBlockProfileRate(r)
		c.String(http.StatusOK, "new rate %d", r)
	})

	service.DebugEngine.GET("/panic", func(c *gin.Context) {
		panic(fmt.Errorf("test panic"))
	})

	service.DebugServer = &graceful.Server{
		Timeout:          service.BaseConfig.ServerShutdownTimeout,
		NoSignalHandling: true,
		Server: &http.Server{
			Handler: service.DebugEngine,
			Addr:    fmt.Sprintf(":%d", service.BaseConfig.Port+9),
		},
	}

	service.DebugServer.ReadTimeout = service.BaseConfig.ServerConnectionTimeout
	service.DebugServer.WriteTimeout = service.BaseConfig.ServerConnectionTimeout

	service.Log.Infof("start debug server listen %s", service.DebugServer.Server.Addr)
	MayPanic(service.DebugServer.ListenAndServe())
}

func (service *Service) ShutdownServers() {
	var serverChan, tlsServerChan, debugServerChan <-chan struct{}

	if service.TlsServer != nil {
		service.Log.Infof("shutting down tls server")
		tlsServerChan = service.TlsServer.StopChan()
		service.TlsServer.Stop(service.BaseConfig.ServerShutdownTimeout)
	}

	if service.Server != nil {
		service.Log.Infof("shutting down server")
		serverChan = service.Server.StopChan()
		service.Server.Stop(service.BaseConfig.ServerShutdownTimeout)
	}

	if service.DebugServer != nil {
		service.Log.Infof("shutting down debug server")
		debugServerChan = service.DebugServer.StopChan()
		service.DebugServer.Stop(service.BaseConfig.ServerShutdownTimeout)
	}

	if service.TlsServer != nil {
		<-tlsServerChan
		service.Log.Infof("tls server shutdown complete")
		service.TlsServer = nil
	}

	if service.Server != nil {
		<-serverChan
		service.Log.Infof("server shutdown complete")
		service.Server = nil
	}

	if service.DebugServer != nil {
		<-debugServerChan
		service.Log.Infof("debug server shutdown complete")
		service.DebugServer = nil
	}
}

func (service *Service) Shutdown() {
	service.Log.Infof("service shutdown")

	service.ShutdownServers()

	if service.MetricsTicker != nil {
		service.Log.Infof("shutting down metrics ticker")
		service.MetricsTicker.Stop()
	}

	if service.Tracker != nil {
		service.Log.Infof("shutting down tracker")
		service.Tracker.Close()
	}

	service.Log.Infof("waiting for rollbar")
	rollbar.Wait()
}

func (service *Service) Wait(shutdownCallback func()) (syscall.Signal, error) {
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
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
	_ = rp.Lookup("goroutine").WriteTo(os.Stdout, 1)
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
