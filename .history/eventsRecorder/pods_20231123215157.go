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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin
	},
}

// Define the list of resource types to watch
var resourceTypes = []string{
	"pods", "services", "deployments", "replicasets", "configmaps", "secrets",
}

func serveWs(w http.ResponseWriter, r *http.Request, clientset *kubernetes.Clientset) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Error upgrading to WebSocket: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("Connected to WebSocket client")

	// Watch events for each resource type concurrently
	for _, resourceType := range resourceTypes {
		go watchResourceEvents(clientset, conn, resourceType)
	}

	// Block indefinitely to keep the WebSocket connection alive
	select {}
}

// Function to watch events for a specific resource type
func watchResourceEvents(clientset *kubernetes.Clientset, conn *websocket.Conn, resourceType string) {
	resourceWatchInterface, err := clientset.CoreV1().RESTClient().Get().
		Resource(resourceType).
		Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error watching %s: %v\n", resourceType, err)
		return
	}

	defer resourceWatchInterface.Stop()

	for event := range resourceWatchInterface.ResultChan() {
		jsonEvent, err := json.Marshal(event.Object)
		if err != nil {
			fmt.Printf("Error encoding event: %v\n", err)
			continue
		}

		fmt.Printf("%s Event received:\n", resourceType)
		fmt.Println(string(jsonEvent))

		// Send the event as JSON to the WebSocket client
		if err := conn.WriteMessage(websocket.TextMessage, jsonEvent); err != nil {
			fmt.Printf("Error writing message: %v\n", err)
			return
		}
	}
	fmt.Printf("Disconnected from %s event stream\n", resourceType)
}

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Printf("Error building kubeconfig: %v\n", err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Error creating Kubernetes clientset: %v\n", err)
		return
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(w, r, clientset)
	})

	fmt.Println("WebSocket server started on :7008")
	if err := http.ListenAndServe(":7008", nil); err != nil {
		fmt.Printf("ListenAndServe error: %v\n", err)
	}
}
