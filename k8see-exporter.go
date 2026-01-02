package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"
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
	// metricsServerReadHeaderTimeout is the timeout for reading request headers in the metrics server.
	metricsServerReadHeaderTimeout = 5 * time.Second

	// Circuit breaker state values.
	circuitBreakerStateClosed   = 0
	circuitBreakerStateHalfOpen = 1
	circuitBreakerStateOpen     = 2

	// Metrics configuration.
	maxRetryAttemptsHistogram = 10
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

	// Async processing fields.
	eventChannel    chan Event
	workerWg        sync.WaitGroup
	shutdownChan    chan struct{}
	shutdownTimeout time.Duration
	numWorkers      int

	// Circuit breaker.
	circuitBreaker *gobreaker.CircuitBreaker
	cbEnabled      bool

	// Backoff configuration.
	backoffConfig *BackoffConfig
}

// BackoffConfig holds exponential backoff settings.
type BackoffConfig struct {
	InitialInterval time.Duration
	Multiplier      float64
	MaxInterval     time.Duration
	MaxElapsedTime  time.Duration
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
	if fileConfigName != "" {
		return loadConfigurationFromFile(fileConfigName)
	}
	return loadConfigurationFromEnv()
}

func loadConfigurationFromFile(fileConfigName string) YamlConfig {
	cfg, err := ReadYAMLConfigFile(fileConfigName)
	if err != nil {
		log.Fatal(err)
	}
	if cfg.MetricsPort == "" {
		cfg.MetricsPort = "2112"
	}
	cfg.SetDefaults()
	return cfg
}

func loadConfigurationFromEnv() YamlConfig {
	log.Infoln("No config file specified. Try to get configuration with environment variable")
	var cfg YamlConfig

	loadBasicConfigFromEnv(&cfg)
	loadAsyncConfigFromEnv(&cfg)
	loadCircuitBreakerConfigFromEnv(&cfg)
	loadBackoffConfigFromEnv(&cfg)

	cfg.SetDefaults()

	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}
	return cfg
}

func loadBasicConfigFromEnv(cfg *YamlConfig) {
	cfg.RedisHost = os.Getenv("REDIS_HOST")
	cfg.RedisPort = os.Getenv("REDIS_PORT")
	cfg.RedisPassword = os.Getenv("REDIS_PASSWORD")
	cfg.RedisStream = os.Getenv("REDIS_STREAM")

	maxStreamLength := os.Getenv("REDIS_STREAM_MAX_LENGTH")
	if maxStreamLength == "" {
		cfg.RedisStreamMaxLength = defaultRedisStreamMaxLength
	} else {
		val, err := strconv.Atoi(maxStreamLength)
		if err != nil {
			log.Fatal(err)
		}
		cfg.RedisStreamMaxLength = val
	}

	cfg.MetricsPort = os.Getenv("METRICS_PORT")
	if cfg.MetricsPort == "" {
		cfg.MetricsPort = "2112"
	}
}

func loadAsyncConfigFromEnv(cfg *YamlConfig) {
	if eventBufferSize := os.Getenv("EVENT_BUFFER_SIZE"); eventBufferSize != "" {
		val, err := strconv.Atoi(eventBufferSize)
		if err != nil {
			log.Fatal(err)
		}
		cfg.EventBufferSize = val
	}
	if eventWorkers := os.Getenv("EVENT_WORKERS"); eventWorkers != "" {
		val, err := strconv.Atoi(eventWorkers)
		if err != nil {
			log.Fatal(err)
		}
		cfg.EventWorkers = val
	}
	if shutdownTimeout := os.Getenv("SHUTDOWN_TIMEOUT_SEC"); shutdownTimeout != "" {
		val, err := strconv.Atoi(shutdownTimeout)
		if err != nil {
			log.Fatal(err)
		}
		cfg.ShutdownTimeout = val
	}
}

func loadCircuitBreakerConfigFromEnv(cfg *YamlConfig) {
	if cbEnabled := os.Getenv("CIRCUIT_BREAKER_ENABLED"); cbEnabled != "" {
		val, err := strconv.ParseBool(cbEnabled)
		if err != nil {
			log.Fatal(err)
		}
		cfg.CircuitBreakerEnabled = val
	}
	if cbMaxRequests := os.Getenv("CIRCUIT_BREAKER_MAX_REQUESTS"); cbMaxRequests != "" {
		val, err := strconv.ParseUint(cbMaxRequests, 10, 32)
		if err != nil {
			log.Fatal(err)
		}
		cfg.CircuitBreakerMaxRequests = uint32(val)
	}
	loadCircuitBreakerTimingFromEnv(cfg)
}

func loadCircuitBreakerTimingFromEnv(cfg *YamlConfig) {
	if cbInterval := os.Getenv("CIRCUIT_BREAKER_INTERVAL_SEC"); cbInterval != "" {
		val, err := strconv.Atoi(cbInterval)
		if err != nil {
			log.Fatal(err)
		}
		cfg.CircuitBreakerInterval = val
	}
	if cbTimeout := os.Getenv("CIRCUIT_BREAKER_TIMEOUT_SEC"); cbTimeout != "" {
		val, err := strconv.Atoi(cbTimeout)
		if err != nil {
			log.Fatal(err)
		}
		cfg.CircuitBreakerTimeout = val
	}
	if cbFailureRatio := os.Getenv("CIRCUIT_BREAKER_FAILURE_RATIO"); cbFailureRatio != "" {
		val, err := strconv.ParseFloat(cbFailureRatio, 64)
		if err != nil {
			log.Fatal(err)
		}
		cfg.CircuitBreakerFailureRatio = val
	}
}

func loadBackoffConfigFromEnv(cfg *YamlConfig) {
	if backoffInitial := os.Getenv("BACKOFF_INITIAL_INTERVAL_MS"); backoffInitial != "" {
		val, err := strconv.Atoi(backoffInitial)
		if err != nil {
			log.Fatal(err)
		}
		cfg.BackoffInitialInterval = val
	}
	if backoffMultiplier := os.Getenv("BACKOFF_MULTIPLIER"); backoffMultiplier != "" {
		val, err := strconv.ParseFloat(backoffMultiplier, 64)
		if err != nil {
			log.Fatal(err)
		}
		cfg.BackoffMultiplier = val
	}
	if backoffMaxInterval := os.Getenv("BACKOFF_MAX_INTERVAL_MS"); backoffMaxInterval != "" {
		val, err := strconv.Atoi(backoffMaxInterval)
		if err != nil {
			log.Fatal(err)
		}
		cfg.BackoffMaxInterval = val
	}
	if backoffMaxElapsed := os.Getenv("BACKOFF_MAX_ELAPSED_TIME_MS"); backoffMaxElapsed != "" {
		val, err := strconv.Atoi(backoffMaxElapsed)
		if err != nil {
			log.Fatal(err)
		}
		cfg.BackoffMaxElapsedTime = val
	}
}

func startMetricsServer(port string) {
	http.Handle("/metrics", promhttp.Handler())
	addr := ":" + port
	log.WithFields(logrus.Fields{
		"address": addr,
	}).Info("Starting metrics server")

	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: metricsServerReadHeaderTimeout,
	}

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Metrics server failed")
	}
}

func setupKubernetesClient() (*kubernetes.Clientset, error) {
	kubeconfig := ""
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build Kubernetes config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	return clientset, nil
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
			log.WithFields(logrus.Fields{
				"eventTime":      e.EventTime,
				"type":           e.Type,
				"reason":         e.Reason,
				"name":           e.Name,
				"firstTimestamp": e.FirstTimestamp,
				"message":        e.Message,
				"uid":            e.UID,
				"namespace":      e.Namespace,
			}).Debug("Kubernetes event added")

			// Track event metrics
			eventsTotal.WithLabelValues(e.Type, e.Namespace).Inc()

			eventTime := time.Unix(e.EventTime.ProtoMicroTime().Seconds, int64(e.EventTime.ProtoMicroTime().Nanos))
			firstTime := e.FirstTimestamp.Time
			log.WithFields(logrus.Fields{
				"eventTime": eventTime.String(),
				"firstTime": firstTime.String(),
			}).Debug("Processing event timestamps")
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

			// Async enqueue to channel (non-blocking)
			select {
			case app.eventChannel <- event:
				eventsEnqueuedTotal.Inc()
				log.WithFields(logrus.Fields{
					"namespace": event.Namespace,
					"name":      event.Name,
				}).Debug("Event enqueued")
			default:
				// Buffer full - drop event
				eventsDroppedTotal.Inc()
				log.WithFields(logrus.Fields{
					"namespace": event.Namespace,
					"name":      event.Name,
				}).Warn("Event buffer full, dropping event")
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

	// Start metrics server in background
	go startMetricsServer(cfg.MetricsPort)

	app, err := NewApp(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	clientset, err := setupKubernetesClient()
	if err != nil {
		log.Fatalf("Failed to setup Kubernetes client: %v", err)
	}

	stop := setupInformerAndWaitForSync(clientset, app)

	waitForShutdownSignal()

	close(stop)

	if err := app.Shutdown(); err != nil {
		log.Errorf("Error during shutdown: %v", err)
		os.Exit(1)
	}

	log.Infoln("Shutdown complete")
}

func setupInformerAndWaitForSync(clientset *kubernetes.Clientset, app *AppK8sEvents2Redis) chan struct{} {
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(clientset, informerResyncInterval)
	if err := setupEventHandler(kubeInformerFactory, app); err != nil {
		log.Fatalf("Failed to setup event handler: %v", err)
	}

	stop := make(chan struct{})
	kubeInformerFactory.Start(stop)

	log.Infoln("Waiting for informer cache to sync...")
	if !cache.WaitForCacheSync(stop, kubeInformerFactory.Core().V1().Events().Informer().HasSynced) {
		close(stop)
		informerSynced.Set(0)
		log.Fatal("Failed to sync informer cache")
	}
	informerSynced.Set(1)
	log.Infoln("Informer cache synced successfully")

	return stop
}

func waitForShutdownSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Infoln("k8see-exporter started. Press Ctrl+C to stop.")
	<-sigChan
	log.Infoln("Received shutdown signal. Shutting down gracefully...")
}

// NewApp is the factory, return an error if the connection to redis server failed.
func NewApp(cfg YamlConfig) (*AppK8sEvents2Redis, error) {
	app := &AppK8sEvents2Redis{
		redisHost:            cfg.RedisHost,
		redisPort:            cfg.RedisPort,
		redisPassword:        cfg.RedisPassword,
		redisStream:          cfg.RedisStream,
		redisMaxStreamLength: cfg.RedisStreamMaxLength,

		// Initialize async processing components
		eventChannel:    make(chan Event, cfg.EventBufferSize),
		shutdownChan:    make(chan struct{}),
		shutdownTimeout: time.Duration(cfg.ShutdownTimeout) * time.Second,
		numWorkers:      cfg.EventWorkers,
		cbEnabled:       cfg.CircuitBreakerEnabled,

		// Initialize backoff configuration
		backoffConfig: &BackoffConfig{
			InitialInterval: time.Duration(cfg.BackoffInitialInterval) * time.Millisecond,
			Multiplier:      cfg.BackoffMultiplier,
			MaxInterval:     time.Duration(cfg.BackoffMaxInterval) * time.Millisecond,
			MaxElapsedTime:  time.Duration(cfg.BackoffMaxElapsedTime) * time.Millisecond,
		},
	}

	// Initialize circuit breaker if enabled
	if cfg.CircuitBreakerEnabled {
		app.circuitBreaker = gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        "RedisWrite",
			MaxRequests: cfg.CircuitBreakerMaxRequests,
			Interval:    time.Duration(cfg.CircuitBreakerInterval) * time.Second,
			Timeout:     time.Duration(cfg.CircuitBreakerTimeout) * time.Second,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
				return counts.Requests >= 3 && failureRatio >= cfg.CircuitBreakerFailureRatio
			},
			OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
				log.WithFields(logrus.Fields{
					"circuitBreaker": name,
					"fromState":      from.String(),
					"toState":        to.String(),
				}).Warn("Circuit breaker state changed")
				// Update metrics
				switch to {
				case gobreaker.StateClosed:
					circuitBreakerState.Set(circuitBreakerStateClosed)
				case gobreaker.StateHalfOpen:
					circuitBreakerState.Set(circuitBreakerStateHalfOpen)
				case gobreaker.StateOpen:
					circuitBreakerState.Set(circuitBreakerStateOpen)
					circuitBreakerTrips.Inc()
				}
			},
		})
	}

	// Set buffer capacity metric
	eventBufferCapacity.Set(float64(cfg.EventBufferSize))

	// Initial Redis connection
	if err := app.InitProducer(); err != nil {
		return nil, err
	}

	// Start worker pool
	app.StartWorkers()

	return app, nil
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

	// Track ping operation duration
	timer := prometheus.NewTimer(redisOperationDuration.WithLabelValues("ping"))
	_, err := a.redisClient.Ping(ctx).Result()
	timer.ObserveDuration()

	if err != nil {
		redisConnected.Set(0)
		return fmt.Errorf("failed to ping Redis server: %w", err)
	}

	redisConnected.Set(1)
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

// StartWorkers launches the background worker pool.
func (a *AppK8sEvents2Redis) StartWorkers() {
	log.WithFields(logrus.Fields{
		"numWorkers": a.numWorkers,
	}).Info("Starting event processing workers")
	activeWorkers.Set(float64(a.numWorkers))

	for i := range a.numWorkers {
		a.workerWg.Add(1)
		go a.eventWorker(i)
	}
}

//nolint:funcorder // Internal async methods grouped together
// eventWorker processes events from the channel.
func (a *AppK8sEvents2Redis) eventWorker(id int) {
	defer a.workerWg.Done()
	log.WithFields(logrus.Fields{
		"workerId": id,
	}).Debug("Worker started")

	for {
		select {
		case event, ok := <-a.eventChannel:
			if !ok {
				log.WithFields(logrus.Fields{
					"workerId": id,
				}).Debug("Worker channel closed, exiting")
				return
			}

			// Update buffer metrics
			bufferLen := len(a.eventChannel)
			eventBufferSize.Set(float64(bufferLen))
			bufferCap := cap(a.eventChannel)
			if bufferCap > 0 {
				eventBufferUtilization.Set(float64(bufferLen) / float64(bufferCap))
			}

			// Process event with retries
			if err := a.writeEventWithRetry(event); err != nil {
				log.WithFields(logrus.Fields{
					"workerId":  id,
					"error":     err.Error(),
					"namespace": event.Namespace,
					"name":      event.Name,
				}).Error("Failed to write event after retries")
				eventsFailedTotal.Inc()
			}

		case <-a.shutdownChan:
			log.WithFields(logrus.Fields{
				"workerId": id,
			}).Debug("Worker shutdown signal received, exiting")
			return
		}
	}
}

//nolint:funcorder // Internal async methods grouped together
// writeEventWithRetry wraps writeToRedis with circuit breaker and exponential backoff.
func (a *AppK8sEvents2Redis) writeEventWithRetry(event Event) error {
	// Create exponential backoff
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = a.backoffConfig.InitialInterval
	b.Multiplier = a.backoffConfig.Multiplier
	b.MaxInterval = a.backoffConfig.MaxInterval
	b.MaxElapsedTime = a.backoffConfig.MaxElapsedTime

	var attempts int
	startTime := time.Now()

	operation := func() error {
		attempts++

		// Use circuit breaker if enabled
		if a.cbEnabled {
			_, err := a.circuitBreaker.Execute(func() (any, error) {
				return nil, a.writeToRedis(event)
			})
			if err != nil {
				return fmt.Errorf("circuit breaker execution failed: %w", err)
			}
			return nil
		}

		// Direct write if circuit breaker disabled
		return a.writeToRedis(event)
	}

	err := backoff.Retry(operation, b)

	// Record retry metrics
	redisRetryAttemptsHistogram.Observe(float64(attempts))
	if attempts > 1 {
		backoffDuration := time.Since(startTime).Seconds()
		redisBackoffDuration.Observe(backoffDuration)
		log.WithFields(logrus.Fields{
			"attempts": attempts,
			"duration": backoffDuration,
		}).Info("Event written successfully after retries")
	}

	if err != nil {
		return fmt.Errorf("retry operation failed after %d attempts: %w", attempts, err)
	}

	return nil
}

//nolint:funcorder // Internal async methods grouped together
// writeToRedis performs the actual Redis write operation.
func (a *AppK8sEvents2Redis) writeToRedis(event Event) error {
	timer := prometheus.NewTimer(eventWriteDuration)
	defer timer.ObserveDuration()

	ctx, cancel := context.WithTimeout(context.Background(), redisOperationTimeout)
	defer cancel()

	opTimer := prometheus.NewTimer(redisOperationDuration.WithLabelValues("xadd"))
	err := a.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: a.redisStream,
		MaxLen: int64(a.redisMaxStreamLength),
		ID:     "",
		Values: map[string]any{
			"name":         event.Name,
			"namespace":    event.Namespace,
			"reason":       event.Reason,
			"type":         event.Type,
			"message":      event.Message,
			"eventTime":    event.EventTime,
			"firstTime":    event.FirstTime,
			"exportedTime": event.ExportedTime,
		},
	}).Err()
	opTimer.ObserveDuration()

	if err != nil {
		log.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Redis XAdd operation failed")
		redisConnected.Set(0)

		// Attempt reconnection
		redisReconnectionsTotal.Inc()
		if reinitErr := a.InitProducer(); reinitErr != nil {
			log.WithFields(logrus.Fields{
				"error": reinitErr.Error(),
			}).Error("Failed to reinitialize Redis connection")
			return fmt.Errorf("redis write failed and reconnection failed: %w", err)
		}

		return fmt.Errorf("redis write operation failed: %w", err)
	}

	eventsWrittenTotal.Inc()
	return nil
}

// Shutdown performs graceful shutdown with timeout.
func (a *AppK8sEvents2Redis) Shutdown() error {
	log.Infoln("Starting graceful shutdown...")

	// Close event channel (no more events accepted)
	close(a.eventChannel)
	log.Infoln("Event channel closed, draining remaining events...")

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		a.workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Infoln("All events processed successfully")
	case <-time.After(a.shutdownTimeout):
		log.WithFields(logrus.Fields{
			"remainingEvents": len(a.eventChannel),
		}).Warn("Shutdown timeout reached, events may be lost")
		close(a.shutdownChan) // Force worker exit

		// Wait a bit for workers to exit
		time.Sleep(1 * time.Second)
	}

	// Close Redis connection
	if err := a.Close(); err != nil {
		return fmt.Errorf("failed to close Redis connection: %w", err)
	}

	// Update metrics
	activeWorkers.Set(0)

	return nil
}

// Write2Stream writes a kubernetes event to the redis stream.
func (a *AppK8sEvents2Redis) Write2Stream(c Event) error {
	timer := prometheus.NewTimer(eventWriteDuration)
	defer timer.ObserveDuration()

	ctx, cancel := context.WithTimeout(context.Background(), redisOperationTimeout)
	defer cancel()

	var err error

	for i := range redisRetryAttempts {
		opTimer := prometheus.NewTimer(redisOperationDuration.WithLabelValues("xadd"))
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
		opTimer.ObserveDuration()

		if err != nil {
			log.WithFields(logrus.Fields{
				"attempt":      i + 1,
				"maxAttempts":  redisRetryAttempts,
				"error":        err.Error(),
			}).Error("Failed to write event to Redis")
			redisReconnectionsTotal.Inc()
			if reinitErr := a.InitProducer(); reinitErr != nil {
				log.WithFields(logrus.Fields{
					"error": reinitErr.Error(),
				}).Error("Failed to reinitialize Redis connection")
				eventsFailedTotal.Inc()
				return fmt.Errorf("redis write failed and reconnection failed: %w", err)
			}
		} else {
			if i != 0 {
				log.Infoln("XAdd error has been recovered")
			}
			eventsWrittenTotal.Inc()
			return nil
		}
	}

	eventsFailedTotal.Inc()
	return fmt.Errorf("%w: %w", errEventNotWritten, err)
}
