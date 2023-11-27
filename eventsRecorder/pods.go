package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow connections from any origin
		},
	}

	eventListOptions = metav1.ListOptions{
		// You can specify additional field selectors or label selectors here to narrow down the events to watch
	}
)

func serveWs(w http.ResponseWriter, r *http.Request, clientset *kubernetes.Clientset) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Error upgrading to WebSocket: %v\n", err)
		return
	}
	defer conn.Close()

	// Watch events of all types in a separate goroutine
	go watchAllEvents(clientset, conn)

	fmt.Println("Connected to WebSocket client")
	// You can add more initialization logic here if needed

	// Block indefinitely to keep the WebSocket connection alive
	select {}
}

// Function to watch events of all types
func watchAllEvents(clientset *kubernetes.Clientset, conn *websocket.Conn) {
	eventWatchInterface, err := clientset.CoreV1().Events("").Watch(context.Background(), eventListOptions)
	if err != nil {
		fmt.Printf("Error watching Events: %v\n", err)
		return
	}

	for event := range eventWatchInterface.ResultChan() {
		jsonEvent, err := json.Marshal(event)
		if err != nil {
			fmt.Printf("Error encoding event: %v\n", err)
			continue
		}

		fmt.Println("Event received:")
		fmt.Println(string(jsonEvent))

		// Send the event as JSON to the WebSocket client
		if err := conn.WriteMessage(websocket.TextMessage, jsonEvent); err != nil {
			fmt.Printf("Error writing message: %v; exiting message loop\n", err)
			break
		}
	}
	fmt.Println("Disconnected from Event stream")
}

func main() {
	var kubeconfig *string

	// Determine the kubeconfig file path
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// Load the Kubernetes configuration from the kubeconfig file
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		// Handle configuration loading error
		fmt.Printf("Error building kubeconfig: %v\n", err)
		return
	}

	// Create a Kubernetes clientset to interact with the API server
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		// Handle clientset creation error
		fmt.Printf("Error creating Kubernetes clientset: %v\n", err)
		return
	}

	// Define HTTP route to upgrade to WebSocket
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(w, r, clientset)
	})

	// Start the WebSocket server on port 7008
	fmt.Println("WebSocket server started on :7008")
	if err := http.ListenAndServe(":7008", nil); err != nil {
		fmt.Printf("ListenAndServe error: %v\n", err)
	}
}
