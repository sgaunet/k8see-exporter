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
	// defaultRedisStreamMaxLength is the default maximum length of the Redis stream.
	defaultRedisStreamMaxLength = 5000
	// redisRetryAttempts is the number of retry attempts for Redis operations.
	redisRetryAttempts = 2
	// redisOperationTimeout is the timeout for Redis operations.
	redisOperationTimeout = 5 * time.Second
)

// Event represents a Kubernetes event to be exported to Redis.
type Event struct {
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
	log     = logrus.New()
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
		cfg.RedisStreamMaxLength = defaultRedisStreamMaxLength
	} else {
		cfg.RedisStreamMaxLength, err = strconv.Atoi(maxStreamLength)
		if err != nil {
			log.Fatal(err)
		}
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}
	return cfg
}

func setupEventHandler(factory kubeinformers.SharedInformerFactory, app *AppK8sEvents2Redis) error {
	// https://pkg.go.dev/k8s.io/api/events/v1#Event
	eventInformer := factory.Core().V1().Events().Informer()

	_, err := eventInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
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
			event := Event{
				ExportedTime: time.Now().Format(time.RFC3339),
				EventTime:    eventTime.Format(time.RFC3339),
				FirstTime:    firstTime.Format(time.RFC3339),
				Type:         e.Type,
				Reason:       e.Reason,
				Name:         e.Name,
				Message:      e.Message,
				Namespace:    e.Namespace,
			}
			err := app.Write2Stream(event)
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
	kubeInformerFactory.Start(stop)

	// Wait for cache sync
	log.Infoln("Waiting for informer cache to sync...")
	if !cache.WaitForCacheSync(stop, kubeInformerFactory.Core().V1().Events().Informer().HasSynced) {
		close(stop)
		log.Fatal("Failed to sync informer cache")
	}
	log.Infoln("Informer cache synced successfully")

	defer close(stop)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Infoln("k8see-exporter started. Press Ctrl+C to stop.")
	<-sigChan
	log.Infoln("Received shutdown signal. Shutting down gracefully...")

	// Cleanup Redis connection
	if err := app.Close(); err != nil {
		log.Errorf("Error closing Redis connection: %v", err)
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), redisOperationTimeout)
	defer cancel()

	addr := fmt.Sprintf("%s:%s", a.redisHost, a.redisPort)
	a.redisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: a.redisPassword,
	})
	_, err := a.redisClient.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to ping Redis server: %w", err)
	}
	log.Infoln("Connected to Redis server")
	return nil
}

// Close closes the Redis client connection.
func (a *AppK8sEvents2Redis) Close() error {
	if a.redisClient != nil {
		if err := a.redisClient.Close(); err != nil {
			return fmt.Errorf("failed to close Redis client: %w", err)
		}
	}
	return nil
}

// Write2Stream writes a kubernetes event to the redis stream.
func (a *AppK8sEvents2Redis) Write2Stream(c Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), redisOperationTimeout)
	defer cancel()

	var err error

	for i := range redisRetryAttempts {
		err = a.redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: a.redisStream,
			MaxLen: int64(a.redisMaxStreamLength),
			ID:     "",
			Values: map[string]any{
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
			log.Errorf("Failed to write event to Redis (attempt %d/%d): %v", i+1, redisRetryAttempts, err)
			if reinitErr := a.InitProducer(); reinitErr != nil {
				log.Errorf("Failed to reinitialize Redis connection: %v", reinitErr)
				return fmt.Errorf("redis write failed and reconnection failed: %w", err)
			}
		} else {
			if i != 0 {
				log.Infoln("XAdd error has been recovered")
			}
			return nil
		}
	}

	return fmt.Errorf("%w: %w", errEventNotWritten, err)
}
