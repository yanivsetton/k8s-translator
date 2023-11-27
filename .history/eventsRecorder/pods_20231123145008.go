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

	// Define Kubernetes objects and fields to watch
	podListOptions = metav1.ListOptions{
		// Add label selectors or field selectors here to narrow down which pods to watch
	}

	deploymentListOptions = metav1.ListOptions{
		// Add label selectors or field selectors here to narrow down which deployments to watch
	}
)

func serveWs(w http.ResponseWriter, r *http.Request, clientset *kubernetes.Clientset) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Error upgrading to WebSocket: %v\n", err)
		return
	}
	defer conn.Close()

	// Watch Pod and Deployment events in separate goroutines
	go watchPodEvents(clientset, conn)
	go watchDeploymentEvents(clientset, conn)

	fmt.Println("Connected to WebSocket client")
	// You can add more initialization logic here if needed

	// Block indefinitely to keep the WebSocket connection alive
	select {}
}

// Function to watch Pod events
func watchPodEvents(clientset *kubernetes.Clientset, conn *websocket.Conn) {
	podWatchInterface, err := clientset.CoreV1().Pods("").Watch(context.Background(), podListOptions)
	if err != nil {
		fmt.Printf("Error watching Pods: %v\n", err)
		return
	}

	for podEvent := range podWatchInterface.ResultChan() {
		jsonEvent, err := json.Marshal(podEvent)
		if err != nil {
			fmt.Printf("Error encoding event: %v\n", err)
			continue
		}

		fmt.Println("Pod Event received:")
		fmt.Println(string(jsonEvent))

		// Send the Pod event as JSON to the WebSocket client
		if err := conn.WriteMessage(websocket.TextMessage, jsonEvent); err != nil {
			fmt.Printf("Error writing message: %v; exiting message loop\n", err)
			break
		}
	}
	fmt.Println("Disconnected from Pod event stream")
}

// Function to watch Deployment events
func watchDeploymentEvents(clientset *kubernetes.Clientset, conn *websocket.Conn) {
	deploymentWatchInterface, err := clientset.AppsV1().Deployments("").Watch(context.Background(), deploymentListOptions)
	if err != nil {
		fmt.Printf("Error watching Deployments: %v\n", err)
		return
	}

	for deploymentEvent := range deploymentWatchInterface.ResultChan() {
		jsonEvent, err := json.Marshal(deploymentEvent)
		if err != nil {
			fmt.Printf("Error encoding event: %v\n", err)
			continue
		}

		fmt.Println("Deployment Event received:")
		fmt.Println(string(jsonEvent))

		// Send the Deployment event as JSON to the WebSocket client
		if err := conn.WriteMessage(websocket.TextMessage, jsonEvent); err != nil {
			fmt.Printf("Error writing message: %v; exiting message loop\n", err)
			break
		}
	}
	fmt.Println("Disconnected from Deployment event stream")
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
