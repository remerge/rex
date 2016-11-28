package service

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	rp "runtime/pprof"
	"strconv"
	"syscall"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gin-gonic/gin"
	"github.com/juju/loggo"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/rcrowley/go-metrics/exp"
	"github.com/remerge/rex"
	"github.com/remerge/rex/env"
	"github.com/remerge/rex/log"
	"github.com/remerge/rex/rollbar"
	"github.com/spf13/cobra"
	"github.com/tylerb/graceful"
)

type Service struct {
	Name        string
	Description string
	Log         loggo.Logger
	Command     *cobra.Command

	Tracker struct {
		Connect       string
		Tracker       rex.Tracker
		EventMetadata rex.EventMetadata
	}

	Server struct {
		Port   int
		Engine *gin.Engine
		Server *graceful.Server

		ShutdownTimeout   time.Duration
		ConnectionTimeout time.Duration

		Debug struct {
			Port   int
			Engine *gin.Engine
			Server *graceful.Server
		}

		TLS struct {
			Port   int
			Cert   string
			Key    string
			Server *graceful.Server
		}
	}
}

func NewService(name string, port int) *Service {
	service := &Service{}
	service.Name = name
	service.Log = log.GetLogger(name)
	service.Command = service.buildCommand()
	service.Server.Port = port
	return service
}

func (service *Service) Execute() {
	if err := service.Command.Execute(); err != nil {
		os.Exit(-1)
	}
}

func (service *Service) buildCommand() *cobra.Command {
	cmd := &cobra.Command{}

	cmd.Use = service.Name
	cmd.Short = fmt.Sprintf("%s: %s", service.Name, service.Description)

	// global flags for all commands
	flags := cmd.PersistentFlags()

	flags.StringVar(
		&env.Env,
		"environment",
		env.Env,
		"environment to run in (development, test, production)",
	)

	flags.StringVar(
		&service.Tracker.EventMetadata.Cluster,
		"cluster",
		"development",
		"cluster to run in (eu, us, etc)",
	)

	flags.StringVar(
		&rollbar.Token,
		"rollbar-token",
		"",
		"rollbar token",
	)

	logSpec := flags.String(
		"log-spec",
		"<root>=INFO",
		"logger configuration",
	)

	// local service flags
	flags = cmd.Flags()

	flags.StringVar(
		&service.Tracker.Connect,
		"tracker-connect", "0.0.0.0:9092",
		"connect string for tracker",
	)

	flags.IntVar(
		&service.Server.Port,
		"server-port", service.Server.Port,
		"HTTP server port",
	)

	flags.DurationVar(
		&service.Server.ShutdownTimeout,
		"server-shutdown-timeout", 30*time.Second,
		"HTTP server shutdown timeout",
	)

	flags.DurationVar(
		&service.Server.ConnectionTimeout,
		"server-connection-timeout", 2*time.Minute,
		"HTTP connection idle timeout",
	)

	flags.IntVar(
		&service.Server.Debug.Port,
		"server-debug-port", 0,
		"HTTP debug server port (default server-port + 9)",
	)

	flags.IntVar(
		&service.Server.TLS.Port,
		"server-tls-port", 0,
		"HTTPS server port",
	)

	flags.StringVar(
		&service.Server.TLS.Cert,
		"server-tls-cert", "",
		"HTTPS server certificate",
	)

	flags.StringVar(
		&service.Server.TLS.Key,
		"server-tls-key", "",
		"HTTPS server certificate key",
	)

	// version command for deployment
	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "display version and exit",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(rex.CodeVersion)
		},
	})

	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// reset env
		env.Set(env.Env)

		// configure logger
		log.ConfigureLoggers(*logSpec)

		// configure rollbar
		rollbar.Environment = env.Env
		rollbar.CodeVersion = rex.CodeVersion

		// configure tracker
		service.Tracker.EventMetadata.Service = service.Name
		service.Tracker.EventMetadata.Environment = env.Env
		service.Tracker.EventMetadata.Host = rex.GetFQDN()
		service.Tracker.EventMetadata.Release = rex.CodeVersion

		// use all cores by default
		if os.Getenv("GOMAXPROCS") == "" {
			runtime.GOMAXPROCS(runtime.NumCPU())
		}

		// gin mode
		switch env.Env {
		case "production":
			gin.SetMode("release")
		case "test", "testing":
			gin.SetMode("test")
		default:
			if log.GetLogger("").IsTraceEnabled() {
				gin.SetMode("debug")
			} else {
				gin.SetMode("release")
			}
		}
	}

	return cmd
}

func (service *Service) Run() {
	if service.Log.IsTraceEnabled() {
		spew.Dump(service)
	}

	service.Log.Infof(
		"starting service %s in env=%v cluster=%v host=%v",
		service.Tracker.EventMetadata.Service,
		service.Tracker.EventMetadata.Environment,
		service.Tracker.EventMetadata.Cluster,
		service.Tracker.EventMetadata.Host,
	)

	service.Log.Infof("code release=%v build=%v", rex.CodeVersion, rex.CodeBuild)

	var err error
	service.Tracker.Tracker, err = rex.NewKafkaTracker(service.Tracker.Connect, &service.Tracker.EventMetadata)
	rex.MayPanic(err)

	// background jobs for go-metrics
	go service.flushMetrics(10 * time.Second)

	// fallback to server port + 9
	if service.Server.Debug.Port < 1 && service.Server.Port > 0 {
		service.Server.Debug.Port = service.Server.Port + 9
	}

	if service.Server.Debug.Port > 0 {
		go service.ServeDebug(service.Server.Debug.Port)
	}
}

func (service *Service) InitEngine() {
	if service.Server.Engine == nil {
		service.Server.Engine = gin.New()
		service.Server.Engine.Use(
			rollbar.GinRecovery(),
			log.GinLogger(fmt.Sprintf("%s.engine", service.Name)),
		)
	}
}

func (service *Service) Serve(handler http.Handler) {
	if handler == nil {
		service.InitEngine()
		handler = service.Server.Engine
	}

	service.Server.Server = &graceful.Server{
		Timeout:          service.Server.ShutdownTimeout,
		NoSignalHandling: true,
		Server: &http.Server{
			Handler: handler,
			Addr:    fmt.Sprintf(":%d", service.Server.Port),
		},
	}

	service.Server.Server.ReadTimeout = service.Server.ConnectionTimeout
	service.Server.Server.WriteTimeout = service.Server.ConnectionTimeout

	service.Log.Infof("start server listen %s", service.Server.Server.Addr)
	rex.MayPanic(service.Server.Server.ListenAndServe())
}

func (service *Service) ServeTLS(handler http.Handler) {
	if handler == nil {
		service.InitEngine()
		handler = service.Server.Engine
	}

	service.Server.TLS.Server = &graceful.Server{
		Timeout: service.Server.ShutdownTimeout,
		Server: &http.Server{
			Handler: handler,
			Addr:    fmt.Sprintf(":%d", service.Server.TLS.Port),
		},
		NoSignalHandling: true,
	}

	service.Server.TLS.Server.ReadTimeout = service.Server.ConnectionTimeout
	service.Server.TLS.Server.WriteTimeout = service.Server.ConnectionTimeout

	service.Log.Infof("start tls server listen %s", service.Server.TLS.Server.Server.Addr)
	rex.MayPanic(service.Server.TLS.Server.ListenAndServeTLS(service.Server.TLS.Cert, service.Server.TLS.Key))
}

func (service *Service) ServeDebug(port int) {
	if service.Server.Debug.Engine == nil {
		service.Server.Debug.Engine = gin.New()
		service.Server.Debug.Engine.Use(
			rollbar.GinRecovery(),
			log.GinLogger(fmt.Sprintf("%s.debug", service.Name)),
		)
	}

	// dynamic loggo configuration
	service.Server.Debug.Engine.GET("/loggo", log.GetLoggoSpec)
	service.Server.Debug.Engine.POST("/loggo", log.SetLoggoSpec)

	// expvar & go-metrics
	service.Server.Debug.Engine.GET("/vars", gin.WrapH(exp.ExpHandler(metrics.DefaultRegistry)))
	service.Server.Debug.Engine.GET("/metrics", gin.WrapH(exp.ExpHandler(metrics.DefaultRegistry)))

	// wrap pprof in gin
	service.Server.Debug.Engine.GET("/pprof/", gin.WrapF(pprof.Index))
	service.Server.Debug.Engine.GET("/pprof/block", gin.WrapF(pprof.Index))
	service.Server.Debug.Engine.GET("/pprof/cmdline", gin.WrapF(pprof.Cmdline))
	service.Server.Debug.Engine.GET("/pprof/goroutine", gin.WrapF(pprof.Index))
	service.Server.Debug.Engine.GET("/pprof/heap", gin.WrapF(pprof.Index))
	service.Server.Debug.Engine.GET("/pprof/profile", gin.WrapF(pprof.Profile))
	service.Server.Debug.Engine.GET("/pprof/symbol", gin.WrapF(pprof.Symbol))
	service.Server.Debug.Engine.POST("/pprof/symbol", gin.WrapF(pprof.Symbol))
	service.Server.Debug.Engine.GET("/pprof/threadcreate", gin.WrapF(pprof.Index))
	service.Server.Debug.Engine.GET("/pprof/trace", gin.WrapF(pprof.Trace))

	service.Server.Debug.Engine.GET("/blockprof/:rate", func(c *gin.Context) {
		r, err := strconv.Atoi(c.Param("rate"))
		if err != nil {
			c.String(http.StatusOK, "rate invalid %s. %v", c.Param("rate"), err)
			return
		}
		runtime.SetBlockProfileRate(r)
		c.String(http.StatusOK, "new rate %d", r)
	})

	service.Server.Debug.Engine.GET("/panic", func(c *gin.Context) {
		panic(fmt.Errorf("test panic"))
	})

	service.Server.Debug.Server = &graceful.Server{
		Timeout:          service.Server.ShutdownTimeout,
		NoSignalHandling: true,
		Server: &http.Server{
			Handler: service.Server.Debug.Engine,
			Addr:    fmt.Sprintf(":%d", port),
		},
	}

	service.Log.Infof("start debug server listen %s", service.Server.Debug.Server.Server.Addr)
	rex.MayPanic(service.Server.Debug.Server.ListenAndServe())
}

func (service *Service) ShutdownServers() {
	var serverChan, tlsServerChan, debugServerChan <-chan struct{}

	if service.Server.TLS.Server != nil {
		service.Log.Infof("shutting down tls server")
		tlsServerChan = service.Server.TLS.Server.StopChan()
		service.Server.TLS.Server.Stop(service.Server.ShutdownTimeout)
	}

	if service.Server.Server != nil {
		service.Log.Infof("shutting down server")
		serverChan = service.Server.Server.StopChan()
		service.Server.Server.Stop(service.Server.ShutdownTimeout)
	}

	if service.Server.Debug.Server != nil {
		service.Log.Infof("shutting down debug server")
		debugServerChan = service.Server.Debug.Server.StopChan()
		service.Server.Debug.Server.Stop(service.Server.ShutdownTimeout)
	}

	if service.Server.TLS.Server != nil {
		<-tlsServerChan
		service.Log.Infof("tls server shutdown complete")
		service.Server.TLS.Server = nil
	}

	if service.Server.Server != nil {
		<-serverChan
		service.Log.Infof("server shutdown complete")
		service.Server.Server = nil
	}

	if service.Server.Debug.Server != nil {
		<-debugServerChan
		service.Log.Infof("debug server shutdown complete")
		service.Server.Debug.Server = nil
	}
}

func (service *Service) Shutdown() {
	service.Log.Infof("service shutdown")

	service.ShutdownServers()

	if service.Tracker.Tracker != nil {
		service.Log.Infof("shutting down tracker")
		service.Tracker.Tracker.Close()
	}

	service.Log.Infof("waiting for rollbar")
	rollbar.Wait()
}

const (
	SIGHUP  = syscall.SIGHUP
	SIGINT  = syscall.SIGINT
	SIGQUIT = syscall.SIGQUIT
	SIGTERM = syscall.SIGTERM
)

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
