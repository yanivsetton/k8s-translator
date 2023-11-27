package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

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

var (
	upgrader        = websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024, CheckOrigin: func(r *http.Request) bool { return true }}
	clients         = make(map[*websocket.Conn]bool) // Connected clients
	mutex           = &sync.Mutex{}                  // Mutex to protect clients map
	eventQueue      = make([][]byte, 0)              // Queue to store events
	broadcastTicker = time.NewTicker(5 * time.Second)
)

func handleConnections(w http.ResponseWriter, r *http.Request, clientset *kubernetes.Clientset) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	mutex.Lock()
	clients[ws] = true
	mutex.Unlock()

	log.Println("WebSocket client connected")

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			mutex.Lock()
			delete(clients, ws)
			mutex.Unlock()
			log.Println("WebSocket client disconnected")
			break
		}
	}
}

func broadcastEvent(jsonEvent []byte) {
	mutex.Lock()
	defer mutex.Unlock()
	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, jsonEvent)
		if err != nil {
			log.Printf("Error writing to WebSocket: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func rateLimitedBroadcast() {
	for range broadcastTicker.C {
		mutex.Lock()
		if len(eventQueue) > 0 {
			log.Printf("Broadcasting event: %s", string(eventQueue[0]))
			broadcastEvent(eventQueue[0])
			eventQueue = eventQueue[1:]
		} else {
			log.Println("No events to broadcast")
		}
		mutex.Unlock()
	}
}

func watchKubernetesEvents(clientset *kubernetes.Clientset) {
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
				jsonEvent, err := json.Marshal(Event{
					Type:      "ADDED",
					Object:    Object{Kind: event.InvolvedObject.Kind, Name: event.InvolvedObject.Name, Namespace: event.InvolvedObject.Namespace, Message: event.Message},
					Timestamp: event.FirstTimestamp.String(),
				})
				if err != nil {
					log.Printf("Error marshaling event: %v", err)
					return
				}

				mutex.Lock()
				eventQueue = append(eventQueue, jsonEvent)
				mutex.Unlock()

				log.Printf("Event added to queue: %s", jsonEvent)
			},
		},
	)

	stopCh := make(chan struct{})
	defer close(stopCh)
	go controller.Run(stopCh)

	log.Println("Starting to watch Kubernetes events")
}

func main() {
	var config *rest.Config
	var err error

	if _, exists := os.LookupEnv("KUBERNETES_SERVICE_HOST"); exists {
		config, err = rest.InClusterConfig()
	} else {
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

	go rateLimitedBroadcast()
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleConnections(w, r, clientset)
	})

	log.Println("WebSocket server started on :7008")
	err = http.ListenAndServe(":7008", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

	go watchKubernetesEvents(clientset)
}
