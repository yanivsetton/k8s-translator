package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/websocket"
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

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// handleConnections handles incoming WebSocket connections
func handleConnections(w http.ResponseWriter, r *http.Request, clientset *kubernetes.Clientset) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	// Watch for Kubernetes events
	watchList := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(),
		"events",
		metav1.NamespaceAll,
		fields.Everything(),
	)

	_, controller := cache.NewInformer(
		watchList,
		&v1.Event{},
		0, // Duration is set to 0
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
				ws.WriteMessage(websocket.TextMessage, jsonEvent)
			},
		},
	)

	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)

	// Keep the connection open
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

func main() {
	var config *rest.Config
	var err error

	// Check if running inside a cluster
	if _, exists := os.LookupEnv("KUBERNETES_SERVICE_HOST"); exists {
		config, err = rest.InClusterConfig()
	} else {
		// Fallback to kubeconfig file for outside cluster access
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		log.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleConnections(w, r, clientset)
	})

	log.Println("WebSocket server started on :7008")
	err = http.ListenAndServe(":7008", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
