package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// Event represents a formatted Kubernetes event
type Event struct {
	Type      string `json:"type"`
	Object    Object `json:"object"`
	Timestamp string `json:"timestamp"`
}

// Object represents the Kubernetes object involved in the event
type Object struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Message   string `json:"message"`
}

var log = logrus.New()
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func handleConnections(w http.ResponseWriter, r *http.Request, clientset *kubernetes.Clientset) {
	var ws *websocket.Conn
	var err error

	for attempt := 0; attempt < 3; attempt++ {
		ws, err = upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.WithFields(logrus.Fields{
				"attempt": attempt + 1,
				"error":   err,
			}).Warning("WebSocket upgrade failed, retrying...")
			time.Sleep(time.Second * 5)
			continue
		}
		break
	}

	if err != nil {
		log.WithField("error", err).Error("WebSocket upgrade failed after retries")
		return
	}

	defer ws.Close()

	watchList := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"events",
		metav1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchList,
		&v1.Event{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				event, ok := obj.(*v1.Event)
				if !ok {
					return
				}
				jsonEvent, _ := json.Marshal(Event{
					Type:      "ADDED",
					Object:    Object{Kind: event.InvolvedObject.Kind, Name: event.InvolvedObject.Name, Namespace: event.InvolvedObject.Namespace, Message: event.Message},
					Timestamp: event.FirstTimestamp.String(),
				})

				log.WithField("event", string(jsonEvent)).Info("New Kubernetes Event")
				ws.WriteMessage(websocket.TextMessage, jsonEvent)
			},
		},
	)

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.WithField("error", err).Warning("WebSocket read error, attempting reconnection")
			handleConnections(w, r, clientset)
			return
		}
	}
}

func main() {
	log.Formatter = &logrus.JSONFormatter{}
	log.Level = logrus.InfoLevel

	var config *rest.Config
	var err error

	if _, exists := os.LookupEnv("KUBERNETES_SERVICE_HOST"); exists {
		config, err = rest.InClusterConfig()
	} else {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		log.WithField("error", err).Fatal("Failed to configure Kubernetes client")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.WithField("error", err).Fatal("Failed to create Kubernetes client")
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleConnections(w, r, clientset)
	})

	log.Info("WebSocket server started on :7008")
	err = http.ListenAndServe(":7008", nil)
	if err != nil {
		log.WithField("error", err).Fatal("ListenAndServe failed")
	}
}
