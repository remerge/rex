package rex

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/getsentry/raven-go"
	"github.com/juju/loggo"
	"github.com/mailgun/manners"
	"github.com/rcrowley/go-metrics"
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
	service.MetricsTicker = NewMetricsTicker(service.Tracker)
	go service.MetricsTicker.Start()

	if service.BaseConfig.Port > 0 {
		service.DebugServer = StartDebugServer(service.BaseConfig.Port + 9)

		listener, err := newListener()
		if err != nil {
			listener, err = net.Listen("tcp", fmt.Sprintf(":%d", service.BaseConfig.Port))
			MayPanic(err)
			service.Log.Infof("start listen=:%d", service.BaseConfig.Port)
		} else {
			service.Log.Infof("resume listen=%s", listener.Addr())
		}

		service.Listener = listener
	}
}

func (service *Service) Serve(handler http.Handler) {
	service.Server = manners.NewWithServer(&http.Server{
		Handler: handler,
	})

	service.Log.Debugf("now serving requests on listener")
	MayPanic(service.Server.Serve(service.Listener))
	service.Log.Debugf("stopped serving on listener")
}

func (service *Service) CloseWait() {
	if service.Server != nil {
		service.Log.Debugf("shutting down http server")
		service.Server.Close()
		service.Listener = nil
		service.Log.Debugf("waiting for requests to finish")
		service.Server.Wait()
		service.Server = nil
		service.Log.Debugf("all request handlers done")
	}
	if service.Listener != nil {
		service.Log.Debugf("closing dangling listener")
		CaptureError(service.Listener.Close())
	}
	if service.DebugServer != nil {
		service.Log.Debugf("shutting down debug server")
		service.DebugServer.Close()
		service.Log.Debugf("waiting for requests to finish")
		service.DebugServer.Wait()
		service.DebugServer = nil
		service.Log.Debugf("all request handlers done")
	}
}

func (service *Service) Shutdown() {
	service.Log.Debugf("service shutdown")
	service.CloseWait()
	service.Log.Debugf("shutting down metrics")
	metrics.Shutdown()
	service.Log.Debugf("shutting down metrics ticker")
	service.MetricsTicker.Stop()
	service.Log.Debugf("shutting down tracker")
	service.Tracker.Close()
	service.Log.Debugf("closing raven client")

	// unfortunately sarama does not expose a sync close
	service.Log.Debugf("give sarama some time to shut down brokers")
	time.Sleep(1 * time.Second)

	// finally close raven and shut down
	Raven.Close()

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
		syscall.SIGTERM,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
	)
	forked := false
	for {
		sig := <-ch
		service.Log.Infof("caught signal %s", sig.String())
		switch sig {

		case syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM:
			service.Log.Infof("shutting down")
			shutdownCallback()
			return sig.(syscall.Signal), nil

		case syscall.SIGUSR2:
			service.Log.Infof("re-executing binary")
			if forked {
				return syscall.SIGUSR2, nil
			}
			forked = true
			if service.ReloadCallback != nil {
				service.ReloadCallback()
			}
			if err := service.ForkExec(); err != nil {
				MayPanic(err)
			}
		}
	}
}

func (service *Service) ForkExec() error {
	argv0, err := lookPath()
	if err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	fd, err := setEnvs(service.Listener)
	if err != nil {
		return err
	}
	if err := os.Setenv("GOAGAIN_PID", ""); err != nil {
		return err
	}
	if err := os.Setenv("GOAGAIN_PPID", fmt.Sprint(syscall.Getpid())); err != nil {
		return err
	}
	var sig syscall.Signal
	sig = syscall.SIGQUIT
	if err := os.Setenv("GOAGAIN_SIGNAL", fmt.Sprintf("%d", sig)); err != nil {
		return err
	}
	files := make([]*os.File, fd+1)
	files[syscall.Stdin] = os.Stdin
	files[syscall.Stdout] = os.Stdout
	files[syscall.Stderr] = os.Stderr
	addr := service.Listener.Addr()
	files[fd] = os.NewFile(
		fd,
		fmt.Sprintf("%s:%s->", addr.Network(), addr.String()),
	)
	service.DebugServer.Close()
	service.DebugServer = nil
	p, err := os.StartProcess(argv0, os.Args, &os.ProcAttr{
		Dir:   wd,
		Env:   os.Environ(),
		Files: files,
		Sys:   &syscall.SysProcAttr{},
	})
	if err != nil {
		return err
	}
	service.Log.Infof("spawned child %d", p.Pid)
	if err = os.Setenv("GOAGAIN_PID", fmt.Sprint(p.Pid)); err != nil {
		return err
	}
	return nil
}

func (service *Service) KillOld() error {
	var (
		pid int
		sig syscall.Signal
	)
	_, err := fmt.Sscan(os.Getenv("GOAGAIN_PID"), &pid)
	if io.EOF == err {
		_, err = fmt.Sscan(os.Getenv("GOAGAIN_PPID"), &pid)
	}
	if nil != err {
		return err
	}
	if _, err := fmt.Sscan(os.Getenv("GOAGAIN_SIGNAL"), &sig); nil != err {
		sig = syscall.SIGQUIT
	}
	service.Log.Infof("sending signal %d to process %d", sig, pid)
	return syscall.Kill(pid, sig)
}

func newListener() (l net.Listener, err error) {
	var fd uintptr
	if _, err = fmt.Sscan(os.Getenv("GOAGAIN_FD"), &fd); nil != err {
		return
	}
	l, err = net.FileListener(os.NewFile(fd, os.Getenv("GOAGAIN_NAME")))
	if nil != err {
		return
	}
	switch l.(type) {
	case *net.TCPListener, *net.UnixListener:
	default:
		err = fmt.Errorf(
			"file descriptor is %T not *net.TCPListener or *net.UnixListener",
			l,
		)
		return
	}
	if err = syscall.Close(int(fd)); nil != err {
		return
	}
	return
}

func lookPath() (argv0 string, err error) {
	argv0, err = exec.LookPath(os.Args[0])
	if err != nil {
		return
	}
	if _, err = os.Stat(argv0); err != nil {
		return
	}
	return
}

func setEnvs(l net.Listener) (fd uintptr, err error) {
	v := reflect.ValueOf(l).Elem().FieldByName("fd").Elem()
	fd = uintptr(v.FieldByName("sysfd").Int())
	_, _, e1 := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_SETFD, 0)
	if 0 != e1 {
		err = e1
		return
	}
	if err = os.Setenv("GOAGAIN_FD", fmt.Sprint(fd)); err != nil {
		return
	}
	addr := l.Addr()
	if err = os.Setenv(
		"GOAGAIN_NAME",
		fmt.Sprintf("%s:%s->", addr.Network(), addr.String()),
	); err != nil {
		return
	}
	return
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
