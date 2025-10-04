package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/sirupsen/logrus"
	kubeinformers "k8s.io/client-go/informers"
)

// https://pkg.go.dev/k8s.io/client-go/kubernetes

const (
	// informerResyncInterval is the interval at which the informer will resync.
	informerResyncInterval = 30 * time.Second
)

type k8sEvent struct {
	// e.FirstTimestamp, e.Type, e.Reason, e.Name, e.Message, e.UID
	ExportedTime string `json:"exportedtime"`
	EventTime    string `json:"eventTime"`
	FirstTime    string `json:"firstTime"`
	Type         string `json:"type"`
	Reason       string `json:"reason"`
	Name         string `json:"name"`
	Message      string `json:"message"`
	Namespace    string `json:"namespace"`
}

// AppK8sEvents2Redis handles the export of Kubernetes events to Redis.
type AppK8sEvents2Redis struct {
	redisHost            string
	redisPort            string
	redisPassword        string
	redisStream          string
	redisMaxStreamLength int
	redisClient          *redis.Client
}

var (
	log = logrus.New()
	version = "development"
	// ErrEventNotWritten is returned when an event could not be written to Redis stream.
	errEventNotWritten = errors.New("an event has not been written to the redis stream")
)

func initTrace(debugLevel string) {
	log.SetOutput(os.Stdout)

	switch debugLevel {
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.DebugLevel)
	}
}

func loadConfiguration(fileConfigName string) YamlConfig {
	var cfg YamlConfig
	var err error
	
	if fileConfigName != "" {
		cfg, err = ReadYAMLConfigFile(fileConfigName)
		if err != nil {
			log.Fatal(err)
		}
		return cfg
	}
	
	log.Infoln("No config file specified. Try to get configuration with environment variable")
	cfg.RedisHost = os.Getenv("REDIS_HOST")
	cfg.RedisPort = os.Getenv("REDIS_PORT")
	cfg.RedisPassword = os.Getenv("REDIS_PASSWORD")
	cfg.RedisStream = os.Getenv("REDIS_STREAM")
	maxStreamLength := os.Getenv("REDIS_STREAM_MAX_LENGTH")
	if maxStreamLength == "" {
		cfg.RedisStreamMaxLength = 5000
	} else {
		cfg.RedisStreamMaxLength, err = strconv.Atoi(maxStreamLength)
		if err != nil {
			log.Fatal(err)
		}
	}
	return cfg
}

func setupEventHandler(factory kubeinformers.SharedInformerFactory, app *AppK8sEvents2Redis) error {
	// https://pkg.go.dev/k8s.io/api/events/v1#Event
	svcInformer := factory.Core().V1().Events().Informer()

	_, err := svcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			e, ok := obj.(*v1.Event)
			if !ok {
				log.Errorln("Failed to cast object to v1.Event")
				return
			}
			log.Debugf("ADDED: eventTime=%s Type=%s Reason=%s Name=%s FirstTimestamp=%s Message=%s UID=%s\n",
				e.EventTime, e.Type, e.Reason, e.Name, e.FirstTimestamp, e.Message, e.UID)
			eventTime := time.Unix(e.EventTime.ProtoMicroTime().Seconds, int64(e.EventTime.ProtoMicroTime().Nanos))
			firstTime := e.FirstTimestamp.Time
			log.Debugf("eventTime=%s firstTime=%s", eventTime.String(), firstTime.String())
			example := k8sEvent{
				ExportedTime: time.Now().Format("2006-01-02 15:04:05 -0700 MST"),
				EventTime:    eventTime.String(),
				FirstTime:    firstTime.String(),
				Type:         e.Type,
				Reason:       e.Reason,
				Name:         e.Name,
				Message:      e.Message,
				Namespace:    e.Namespace,
			}
			err := app.Write2Stream(example)
			if err != nil {
				log.Errorln(err.Error())
			}
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}
	return nil
}

func main() {
	var fileConfigName string
	var showVersion bool
	var err error

	flag.StringVar(&fileConfigName, "f", "", "YAML file to parse.")
	flag.BoolVar(&showVersion, "v", false, "Print version and exit.")
	flag.Parse()
	
	if showVersion {
		fmt.Println(version)
		os.Exit(0)
	}
	
	initTrace(os.Getenv("LOGLEVEL"))
	cfg := loadConfiguration(fileConfigName)

	log.Debugf("cfg=%+v\n", cfg)
	app, err := NewApp(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// https://medium.com/swlh/clientset-module-for-in-cluster-and-out-cluster-3f0d80af79ed
	kubeconfig := ""
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Failed to build Kubernetes config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(clientset, informerResyncInterval)
	if err := setupEventHandler(kubeInformerFactory, app); err != nil {
		log.Fatalf("Failed to setup event handler: %v", err)
	}

	stop := make(chan struct{})
	defer close(stop)
	kubeInformerFactory.Start(stop)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Infoln("k8see-exporter started. Press Ctrl+C to stop.")
	<-sigChan
	log.Infoln("Received shutdown signal. Shutting down gracefully...")
}

// NewApp is the factory, return an error if the connection to redis server failed.
func NewApp(cfg YamlConfig) (*AppK8sEvents2Redis, error) {
	app := AppK8sEvents2Redis{
		redisHost:            cfg.RedisHost,
		redisPort:            cfg.RedisPort,
		redisPassword:        cfg.RedisPassword,
		redisStream:          cfg.RedisStream,
		redisMaxStreamLength: cfg.RedisStreamMaxLength,
	}
	return &app, app.InitProducer()
}

// InitProducer initialise redisClient and ensure that connection is ok.
func (a *AppK8sEvents2Redis) InitProducer() error {
	var err error
	ctx := context.TODO()
	addr := fmt.Sprintf("%s:%s", a.redisHost, a.redisPort)
	a.redisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: a.redisPassword,
	})
	_, err = a.redisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to ping Redis server: %w", err)
	}
	log.Infoln("Connected to Redis server")
	return nil
}

// Write2Stream writes a kubernetes event to the redis stream.
func (a *AppK8sEvents2Redis) Write2Stream(c k8sEvent) error {
	const nbtry int = 2
	ctx := context.TODO()
	var err error

	for i := range nbtry {
		err = a.redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: a.redisStream,
			MaxLen: int64(a.redisMaxStreamLength),
			ID:     "",
			Values: map[string]interface{}{
				"name":         c.Name,
				"namespace":    c.Namespace,
				"reason":       c.Reason,
				"type":         c.Type,
				"message":      c.Message,
				"eventTime":    c.EventTime,
				"firstTime":    c.FirstTime,
				"exportedTime": c.ExportedTime,
			},
		}).Err()
		if err != nil {
			log.Errorln(err.Error())
			if err := a.InitProducer(); err != nil {
				log.Errorln("Failed to reinitialize producer:", err)
			}
		} else {
			if i != 0 {
				log.Infoln("XAdd error has been recovered")
			}
			break
		}
	}

	if err != nil {
		return errEventNotWritten
	}

	return nil
}
