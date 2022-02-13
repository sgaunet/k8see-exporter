package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	//v1 "k8s.io/api/events/v1"
	"github.com/go-redis/redis/v7"
	//"github.com/gomodule/redigo/redis"
	"github.com/robinjoseph08/redisqueue/v2"
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
	producer      *redisqueue.Producer
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
	var fileConfigName string
	flag.StringVar(&fileConfigName, "f", "", "YAML file to parse.")
	flag.Parse()

	initTrace(os.Getenv("LOGLEVEL"))

	if fileConfigName == "" {
		log.Fatal("No config file specified.")
		os.Exit(1)
	}

	cfg, err := ReadyamlConfigFile(fileConfigName)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	app := NewApp(cfg.RedisHost, cfg.RedisPort, cfg.RedisPassword, cfg.RedisStream)

	// cmd := exec.Command("sh", "-c", "kubectl get events --watch")
	// // stderr, err := cmd.StderrPipe()
	// stdout, err := cmd.StdoutPipe()
	// if err != nil {
	// 	fmt.Println(err)
	// }

	// err = cmd.Start()
	// fmt.Println("The command is running")
	// if err != nil {
	// 	fmt.Println(err)
	// }

	// // print the output of the subprocess
	// scanner := bufio.NewScanner(stdout)
	// for scanner.Scan() {
	// 	m := scanner.Text()
	// 	fmt.Println(m)
	// }
	// cmd.Wait()

	// log.Print("Server Exited Properly")

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
			// fmt.Printf("======: %s \n", reflect.TypeOf(obj))
			// fmt.Printf("Reason: %s \n", e.Reason)
			// fmt.Printf("EventTime: %v \n", e.EventTime)
			// fmt.Printf("Message: %s \n", e.Message)
			// fmt.Printf("Action: %s \n", e.Action)
			// fmt.Printf("FirstTimestamp: %s \n", e.FirstTimestamp)
			// fmt.Printf("Namespace: %s \n", e.Namespace)
			// fmt.Printf("Name: %s \n", e.Name)
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
			app.Write2Stream(example)
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

func NewApp(redisHost string, redisPort string, redisPassword string, redisStream string) *appK8sEvents2Redis {
	app := appK8sEvents2Redis{
		redisHost:     redisHost,
		redisPort:     redisPort,
		redisPassword: redisPassword,
		redisStream:   redisStream,
	}

	app.InitProducer()
	return &app
}

func (a *appK8sEvents2Redis) InitProducer() {
	var err error
	addr := fmt.Sprintf("%s:%s", a.redisHost, a.redisPort)
	a.producer, err = redisqueue.NewProducerWithOptions(&redisqueue.ProducerOptions{
		//Name:                 "localhost",
		StreamMaxLength:      1000,
		ApproximateMaxLength: true,
		RedisOptions: &redis.Options{
			Addr:     addr,
			Password: a.redisPassword,
			DB:       0, // use default DB
		},
	})
	if err != nil {
		log.Errorln(err)
	}
}

func (a *appK8sEvents2Redis) Write2Stream(c k8sEvent) {
	err := a.producer.Enqueue(&redisqueue.Message{
		Stream: a.redisStream,
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
	})
	if err != nil {
		log.Errorln(err)
	}
}
