package main

import (
	// Importing necessary packages for websockets, logging, Kubernetes interaction, and other utilities.
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

// Event struct defines the structure for Kubernetes events.
type Event struct {
	Type      string `json:"type"`      // Event type (e.g., ADDED, MODIFIED, DELETED)
	Object    Object `json:"object"`    // The Kubernetes object involved in the event
	Timestamp string `json:"timestamp"` // Timestamp of the event
}

// Object struct defines the Kubernetes object involved in the event.
type Object struct {
	Kind      string `json:"kind"`      // Kind of the Kubernetes object (e.g., Pod, Service)
	Name      string `json:"name"`      // Name of the object
	Namespace string `json:"namespace"` // Namespace of the object
	Message   string `json:"message"`   // Event message
}

// Logger instance configured for structured logging.
var log = logrus.New()

// WebSocket upgrader configuration with buffer sizes and open check origin policy.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,                                       // Size of the read buffer for the WebSocket
	WriteBufferSize: 1024,                                       // Size of the write buffer for the WebSocket
	CheckOrigin:     func(r *http.Request) bool { return true }, // Allow connections from any origin
}

// handleConnections manages WebSocket connections and streams Kubernetes events.
func handleConnections(w http.ResponseWriter, r *http.Request, clientset *kubernetes.Clientset) {
	var ws *websocket.Conn
	var err error

	// Retry loop for establishing WebSocket connection.
	for attempt := 0; attempt < 3; attempt++ {
		ws, err = upgrader.Upgrade(w, r, nil) // Attempt to upgrade the HTTP connection to a WebSocket
		if err != nil {
			// Log the failure and retry after a delay.
			log.WithFields(logrus.Fields{
				"attempt": attempt + 1,
				"error":   err,
			}).Warning("WebSocket upgrade failed, retrying...")
			time.Sleep(time.Second * 5) // Wait for 5 seconds before retrying
			continue
		}
		break
	}

	// Log failure after all retries and return.
	if err != nil {
		log.WithField("error", err).Error("WebSocket upgrade failed after retries")
		return
	}

	// Close the WebSocket connection when the function exits.
	defer ws.Close()

	// Setting up a watch on Kubernetes events across all namespaces.
	watchList := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(), // Kubernetes REST client
		"events",                        // Resource to watch
		metav1.NamespaceAll,             // Watch events in all namespaces
		fields.Everything(),             // Select all fields
	)

	// Creating an informer to watch and handle Kubernetes events.
	_, controller := cache.NewInformer(
		watchList,   // The watch list defined earlier
		&v1.Event{}, // The type of object to watch (Kubernetes events)
		0,           // Resync interval (0 means no resync)
		cache.ResourceEventHandlerFuncs{ // Define handling functions for different event types
			AddFunc: func(obj interface{}) {
				event, ok := obj.(*v1.Event) // Type assertion for the event object
				if !ok {
					return
				}
				// Marshal the event into the Event struct and prepare it for JSON encoding.
				jsonEvent, _ := json.Marshal(Event{
					Type:      "ADDED",
					Object:    Object{Kind: event.InvolvedObject.Kind, Name: event.InvolvedObject.Name, Namespace: event.InvolvedObject.Namespace, Message: event.Message},
					Timestamp: event.FirstTimestamp.String(),
				})

				// Log the new Kubernetes event.
				log.WithField("event", string(jsonEvent)).Info("New Kubernetes Event")
				// Send the event to the WebSocket client.
				ws.WriteMessage(websocket.TextMessage, jsonEvent)
			},
		},
	)

	// Channel to signal stopping of the controller.
	stop := make(chan struct{})
	// Ensure the stop channel is closed when the function exits.
	defer close(stop)
	// Run the controller in a separate goroutine.
	go controller.Run(stop)

	// Keep the WebSocket connection open and read messages.
	for {
		_, _, err := ws.ReadMessage() // Read a message from the WebSocket (message content is ignored)
		if err != nil {
			// Log the read error and attempt to re-establish the connection.
			log.WithField("error", err).Warning("WebSocket read error, attempting reconnection")
			handleConnections(w, r, clientset) // Recursive call to re-establish the connection
			return
		}
	}
}

// main is the entry point of the application.
func main() {
	// Configuring the logger for JSON format and setting the log level to Info.
	log.Formatter = &logrus.JSONFormatter{}
	log.Level = logrus.InfoLevel

	var config *rest.Config
	var err error

	// Determining the Kubernetes configuration context (in-cluster or external).
	if _, exists := os.LookupEnv("KUBERNETES_SERVICE_HOST"); exists {
		// In-cluster configuration for Kubernetes client.
		config, err = rest.InClusterConfig()
	} else {
		// External cluster access using kubeconfig file.
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	// Log and exit on failure to configure the Kubernetes client.
	if err != nil {
		log.WithField("error", err).Fatal("Failed to configure Kubernetes client")
	}

	// Setting up the Kubernetes client.
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.WithField("error", err).Fatal("Failed to create Kubernetes client")
	}

	// Registering the WebSocket endpoint handler.
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleConnections(w, r, clientset) // Handle incoming WebSocket connections
	})

	// Start the WebSocket server and listen on port 7008.
	log.Info("WebSocket server started on :7008")
	err = http.ListenAndServe(":7008", nil)
	if err != nil {
		// Log and exit on failure to start the server.
		log.WithField("error", err).Fatal("ListenAndServe failed")
	}
}
