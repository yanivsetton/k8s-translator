package main

import (
	// Importing necessary packages
	"encoding/json" // For JSON encoding
	"net/http"      // HTTP server functionalities
	"os"            // Interface to operating system functionality
	"path/filepath" // For manipulating filename paths
	"time"          // For time-related operations

	"github.com/gorilla/websocket"                // Package for WebSocket implementations
	"github.com/sirupsen/logrus"                  // Package for structured logging
	v1 "k8s.io/api/core/v1"                       // Core v1 API for Kubernetes
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // Meta v1 API for Kubernetes
	"k8s.io/apimachinery/pkg/fields"              // For selecting Kubernetes fields
	"k8s.io/client-go/kubernetes"                 // Kubernetes client
	"k8s.io/client-go/rest"                       // RESTful implementation of Kubernetes API
	"k8s.io/client-go/tools/cache"                // For caching Kubernetes objects
	"k8s.io/client-go/tools/clientcmd"            // For command line configuration of Kubernetes
)

// Event struct defines the structure for Kubernetes events.
type Event struct {
	Type      string `json:"type"`      // Type of the event
	Object    Object `json:"object"`    // Kubernetes object involved in the event
	Timestamp string `json:"timestamp"` // Timestamp of the event
}

// Object struct defines the Kubernetes object involved in the event.
type Object struct {
	Kind      string `json:"kind"`      // Type of Kubernetes object
	Name      string `json:"name"`      // Name of the object
	Namespace string `json:"namespace"` // Kubernetes namespace
	Message   string `json:"message"`   // Event message
}

// Logger instance for structured logging
var log = logrus.New()

// WebSocket upgrader configuration
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,                                       // Read buffer size
	WriteBufferSize: 1024,                                       // Write buffer size
	CheckOrigin:     func(r *http.Request) bool { return true }, // Allowing all origins
}

// handleConnections manages WebSocket connections and streams Kubernetes events
func handleConnections(w http.ResponseWriter, r *http.Request, clientset *kubernetes.Clientset) {
	var ws *websocket.Conn
	var err error

	// Retry loop for establishing WebSocket connection
	for attempt := 0; attempt < 3; attempt++ {
		ws, err = upgrader.Upgrade(w, r, nil) // Upgrading HTTP to WebSocket
		if err != nil {
			// Log the error and retry after a pause
			log.WithFields(logrus.Fields{
				"attempt": attempt + 1,
				"error":   err,
			}).Warning("WebSocket upgrade failed, retrying...")
			time.Sleep(time.Second * 5) // 5-second pause
			continue
		}
		break
	}

	// Handle failure after retries
	if err != nil {
		log.WithField("error", err).Error("WebSocket upgrade failed after retries")
		return
	}

	// Close WebSocket connection on function exit
	defer ws.Close()

	// Setting up Kubernetes event watcher
	watchList := cache.NewListWatchFromClient(
		clientset.CoreV1().RESTClient(), // REST client for events
		"events",                        // Watching events
		metav1.NamespaceAll,             // In all namespaces
		fields.Everything(),             // Selecting all fields
	)

	// Informer for handling Kubernetes events
	_, controller := cache.NewInformer(
		watchList,   // Watch list created above
		&v1.Event{}, // Watching Kubernetes Event objects
		0,           // No resync period
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				event, ok := obj.(*v1.Event) // Casting to *v1.Event
				if !ok {
					return
				}

				// Formatting timestamp to be more human-readable
				formattedTimestamp := event.FirstTimestamp.Time.Format("2006-01-02 15:04:05")

				// Marshaling event data to JSON
				jsonEvent, _ := json.Marshal(Event{
					Type:      "ADDED",
					Object:    Object{Kind: event.InvolvedObject.Kind, Name: event.InvolvedObject.Name, Namespace: event.InvolvedObject.Namespace, Message: event.Message},
					Timestamp: formattedTimestamp,
				})

				// Logging the event
				log.WithField("event", string(jsonEvent)).Info("New Kubernetes Event")
				// Sending event over WebSocket
				ws.WriteMessage(websocket.TextMessage, jsonEvent)
			},
		},
	)

	// Channel to signal the stop of the controller
	stop := make(chan struct{})
	defer close(stop)       // Ensure channel is closed when exiting
	go controller.Run(stop) // Running the controller in a separate goroutine

	// Keeping the WebSocket connection alive
	for {
		_, _, err := ws.ReadMessage() // Reading message (content ignored)
		if err != nil {
			// Log and attempt to re-establish connection
			log.WithField("error", err).Warning("WebSocket read error, attempting reconnection")
			handleConnections(w, r, clientset) // Recursive call for reconnection
			return
		}
	}
}

func main() {
	// Logger configuration
	log.Formatter = &logrus.JSONFormatter{} // JSON formatter for logging
	log.Level = logrus.InfoLevel            // Setting log level to Info

	var config *rest.Config
	var err error

	// Determining Kubernetes configuration context (in-cluster or external)
	if _, exists := os.LookupEnv("KUBERNETES_SERVICE_HOST"); exists {
		config, err = rest.InClusterConfig() // In-cluster configuration
	} else {
		kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config") // Path to kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)      // Building config from kubeconfig
	}

	// Handle configuration error
	if err != nil {
		log.WithField("error", err).Fatal("Failed to configure Kubernetes client")
	}

	// Creating Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.WithField("error", err).Fatal("Failed to create Kubernetes client")
	}

	// Registering WebSocket endpoint
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleConnections(w, r, clientset) // Handling WebSocket connections
	})

	// Starting WebSocket server
	log.Info("WebSocket server started on :7008")
	err = http.ListenAndServe(":7008", nil)
	if err != nil {
		log.WithField("error", err).Fatal("ListenAndServe failed") // Handling server start error
	}
}
