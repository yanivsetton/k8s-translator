package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	v1 "k8s.io/api/core/v1"
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
	watchList := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "events", metav1.NamespaceAll, cache.ResourceEventHandlerFuncs{
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
	})

	stop := make(chan struct{})
	defer close(stop)
	go watchList.ListWatch.Run(stop)

	// Keep the connection open
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

func main() {
	config, err := rest.InClusterConfig()
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

	log.Println("WebSocket server started on :8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
