package rex

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

	"github.com/gin-gonic/gin"
	"github.com/juju/loggo"
	"github.com/remerge/rex/env"
	. "github.com/remerge/rex/log"
	"github.com/remerge/rex/rollbar"
	"github.com/spf13/viper"
	"github.com/tylerb/graceful"
)

var CodeVersion = "unknown"
var CodeBuild = "unknown"

const (
	SIGHUP  = syscall.SIGHUP
	SIGINT  = syscall.SIGINT
	SIGQUIT = syscall.SIGQUIT
	SIGTERM = syscall.SIGTERM
	SIGUSR1 = syscall.SIGUSR1
	SIGUSR2 = syscall.SIGUSR2
)

type Service struct {
	Name          string
	Log           loggo.Logger
	Tracker       Tracker
	MetricsTicker *MetricsTicker
	EventMetadata EventMetadata
	Engine        *gin.Engine
	Server        *graceful.Server
	TlsServer     *graceful.Server
	DebugEngine   *gin.Engine
	DebugServer   *graceful.Server
	GinRecovery   gin.HandlerFunc
}

func (service *Service) InitEngine() {
	if service.Engine == nil {
		service.Engine = gin.New()
		service.Engine.Use(
			GinRecovery(),
			GinLogger(fmt.Sprintf("%s.engine", service.Name)),
		)
	}

	if service.DebugEngine == nil {
		service.DebugEngine = gin.New()
		service.DebugEngine.Use(
			GinRecovery(),
			GinLogger(fmt.Sprintf("%s.debug", service.Name)),
		)
	}
}

func (service *Service) Init() {
	service.Log = GetLogger(service.Name)

	viper.SetDefault("cluster", "development")
	viper.SetDefault("server.shutdown.timeout", 30*time.Second)
	viper.SetDefault("server.connection.timeout", 2*time.Minute)
	viper.SetDefault("tracker.kafka.connect", "0.0.0.0:9092")

	service.EventMetadata.Service = service.Name
	service.EventMetadata.Environment = env.Env
	service.EventMetadata.Cluster = viper.GetString("cluster")
	service.EventMetadata.Host = GetFQDN()
	service.EventMetadata.Release = CodeVersion

	service.InitEngine()
}

func (service *Service) Run() {
	var err error
	service.Tracker, err = NewKafkaTracker(viper.GetString("tracker.kafka.connect"), &service.EventMetadata)
	MayPanic(err)

	service.MetricsTicker = NewMetricsTicker(service.Tracker)
	go service.MetricsTicker.Start()

	if viper.GetInt("port") > 0 {
		go service.ServeDebug()
	}
}

func GinLogger(name string) gin.HandlerFunc {
	log := GetLogger(name)
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Debugf("%s %s -> %d in %v %s",
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			time.Now().Sub(start),
			c.Errors.String(),
		)
	}
}

func (service *Service) Serve(handler http.Handler) {
	if handler == nil {
		handler = service.Engine
	}

	service.Server = &graceful.Server{
		Timeout:          viper.GetDuration("server.shutdown.timeout"),
		NoSignalHandling: true,
		Server: &http.Server{
			Handler: handler,
			Addr:    fmt.Sprintf(":%d", viper.GetInt("port")),
		},
	}

	service.Server.ReadTimeout = viper.GetDuration("server.connection.timeout")
	service.Server.WriteTimeout = viper.GetDuration("server.connection.timeout")

	service.Log.Infof("start server listen %s", service.Server.Server.Addr)
	MayPanic(service.Server.ListenAndServe())
}

func (service *Service) ServeTLS(handler http.Handler) {
	if handler == nil {
		handler = service.Engine
	}

	service.TlsServer = &graceful.Server{
		Timeout: viper.GetDuration("server.shutdown.timeout"),
		Server: &http.Server{
			Handler: handler,
			Addr:    fmt.Sprintf(":%d", viper.GetInt("tls.port")),
		},
		NoSignalHandling: true,
	}

	service.TlsServer.ReadTimeout = viper.GetDuration("server.connection.timeout")
	service.TlsServer.WriteTimeout = viper.GetDuration("server.connection.timeout")

	service.Log.Infof("start tls server listen %s", service.TlsServer.Server.Addr)
	MayPanic(service.TlsServer.ListenAndServeTLS(viper.GetString("tls.cert"), viper.GetString("tls.key")))
}

func (service *Service) ServeDebug() {
	service.DebugEngine.GET("/loggo", getLoggoSpec)
	service.DebugEngine.POST("/loggo", setLoggoSpec)

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
		Timeout:          viper.GetDuration("server.shutdown.timeout"),
		NoSignalHandling: true,
		Server: &http.Server{
			Handler: service.DebugEngine,
			Addr:    fmt.Sprintf(":%d", viper.GetInt("port")+9),
		},
	}

	service.DebugServer.ReadTimeout = viper.GetDuration("server.connection.timeout")
	service.DebugServer.WriteTimeout = viper.GetDuration("server.connection.timeout")

	service.Log.Infof("start debug server listen %s", service.DebugServer.Server.Addr)
	MayPanic(service.DebugServer.ListenAndServe())
}

func (service *Service) ShutdownServers() {
	var serverChan, tlsServerChan, debugServerChan <-chan struct{}

	if service.TlsServer != nil {
		service.Log.Infof("shutting down tls server")
		tlsServerChan = service.TlsServer.StopChan()
		service.TlsServer.Stop(viper.GetDuration("server.shutdown.timeout"))
	}

	if service.Server != nil {
		service.Log.Infof("shutting down server")
		serverChan = service.Server.StopChan()
		service.Server.Stop(viper.GetDuration("server.shutdown.timeout"))
	}

	if service.DebugServer != nil {
		service.Log.Infof("shutting down debug server")
		debugServerChan = service.DebugServer.StopChan()
		service.DebugServer.Stop(viper.GetDuration("server.shutdown.timeout"))
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
