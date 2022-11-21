package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v7"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/sirupsen/logrus"
	kubeinformers "k8s.io/client-go/informers"
)

// https://pkg.go.dev/k8s.io/client-go/kubernetes

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

type appK8sEvents2Redis struct {
	redisHost     string
	redisPort     string
	redisPassword string
	redisStream   string
	redisClient   *redis.Client
}

var log = logrus.New()

func initTrace(debugLevel string) {
	// Log as JSON instead of the default ASCII formatter.
	//log.SetFormatter(&log.JSONFormatter{})
	// log.SetFormatter(&log.TextFormatter{
	// 	DisableColors: true,
	// 	FullTimestamp: true,
	// })

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
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

func main() {
	var cfg YamlConfig
	var fileConfigName string
	var err error

	flag.StringVar(&fileConfigName, "f", "", "YAML file to parse.")
	flag.Parse()
	initTrace(os.Getenv("LOGLEVEL"))

	if fileConfigName != "" {
		cfg, err = ReadyamlConfigFile(fileConfigName)
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
	} else {
		log.Infoln("No config file specified. Try to get configuration with environment variable")
		cfg.RedisHost = os.Getenv("REDIS_HOST")
		cfg.RedisPort = os.Getenv("REDIS_PORT")
		cfg.RedisPassword = os.Getenv("REDIS_PASSWORD")
		cfg.RedisStream = os.Getenv("REDIS_STREAM")
	}

	log.Debugf("cfg=%+v\n", cfg)
	app, err := NewApp(cfg.RedisHost, cfg.RedisPort, cfg.RedisPassword, cfg.RedisStream)
	if err != nil {
		log.Errorln(err.Error())
	}

	// https://medium.com/swlh/clientset-module-for-in-cluster-and-out-cluster-3f0d80af79ed
	kubeconfig := ""
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	// clientset, err := kubernetes.NewForConfig()
	if err != nil {
		panic(err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(clientset, time.Second*30)
	// https://pkg.go.dev/k8s.io/api/events/v1#Event
	svcInformer := kubeInformerFactory.Core().V1().Events().Informer()

	svcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			e := obj.(*v1.Event)
			log.Debugf("ADDED: eventTime=%s Type=%s Reason=%s Name=%s FirstTimestamp=%s Message=%s UID=%s\n", e.EventTime, e.Type, e.Reason, e.Name, e.FirstTimestamp, e.Message, e.UID)
			eventTime := time.Unix(e.EventTime.ProtoMicroTime().Seconds, int64(e.EventTime.ProtoMicroTime().Nanos))
			firstTime := time.Date(e.FirstTimestamp.Year(), e.FirstTimestamp.Month(), e.FirstTimestamp.Day(), e.FirstTimestamp.Hour(), e.FirstTimestamp.Minute(), e.FirstTimestamp.Second(), e.FirstTimestamp.Nanosecond(), e.FirstTimestamp.Location())
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
			err = app.Write2Stream(example)
			if err != nil {
				log.Errorln(err.Error())
			}
		},
		// DeleteFunc: func(obj interface{}) {
		// 	e := obj.(*v1.Event)
		// 	log.Infof("DELETED: %s %s %s %s %s\n", e.FirstTimestamp, e.Type, e.Reason, e.Name, e.Message)
		// },
		// UpdateFunc: func(oldObj, newObj interface{}) {
		// 	e := newObj.(*v1.Event)
		// 	log.Infof("UPDATED: %s %s %s %s %s\n", e.FirstTimestamp, e.Type, e.Reason, e.Name, e.Message)
		// },
	})

	stop := make(chan struct{})
	defer close(stop)
	kubeInformerFactory.Start(stop)
	for {
		time.Sleep(time.Second)
	}
}

// NewAPP is the factory, return an error if the connection to redis server failed
func NewApp(redisHost string, redisPort string, redisPassword string, redisStream string) (*appK8sEvents2Redis, error) {
	app := appK8sEvents2Redis{
		redisHost:     redisHost,
		redisPort:     redisPort,
		redisPassword: redisPassword,
		redisStream:   redisStream,
	}
	return &app, app.InitProducer()
}

// InitProducer initialise redisClient and ensure that connection is ok
func (a *appK8sEvents2Redis) InitProducer() error {
	var err error
	addr := fmt.Sprintf("%s:%s", a.redisHost, a.redisPort)
	a.redisClient = redis.NewClient(&redis.Options{
		Addr: addr,
	})
	_, err = a.redisClient.Ping().Result()
	if err != nil {
		return err
	}
	log.Infoln("Connected to Redis server")
	return nil
}

// Write2Stream writes a kubernetes event to the redis stream
func (a *appK8sEvents2Redis) Write2Stream(c k8sEvent) (err error) {
	const nbtry int = 2

	for i := 0; i < nbtry; i++ {
		err := a.redisClient.XAdd(&redis.XAddArgs{
			Stream:       a.redisStream,
			MaxLen:       0,
			MaxLenApprox: 0,
			ID:           "",
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
			a.InitProducer()
		} else {
			if i != 0 {
				log.Infoln("XAdd error has been recovered")
			}
			break
		}
	}

	if err != nil {
		return errors.New("an event has not been written to the redis stream")
	}

	return err
}
