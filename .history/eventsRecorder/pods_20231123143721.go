package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

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

// Log events to a file
func logEvent(event interface{}, logFile *os.File) {
	jsonEvent, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		fmt.Printf("Error encoding event: %v\n", err)
		return
	}

	fmt.Println("Event received:")
	fmt.Println(string(jsonEvent))

	if _, err := logFile.Write(jsonEvent); err != nil {
		fmt.Printf("Error writing event to log file: %v\n", err)
	}
}

// Function to watch Deployments and log scaling events
func watchDeployments(clientset *kubernetes.Clientset, logFile *os.File) {
	watchInterface, err := clientset.AppsV1().Deployments("").Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error watching Deployments: %v\n", err)
		return
	}

	for event := range watchInterface.ResultChan() {
		logEvent(event, logFile)
	}
}

func serveWs(w http.ResponseWriter, r *http.Request, clientset *kubernetes.Clientset, logFile *os.File) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Error upgrading to WebSocket: %v\n", err)
		return
	}
	defer conn.Close()

	// Watch Pod and Deployment events
	podWatchInterface, err := clientset.CoreV1().Pods("").Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error watching Pods: %v\n", err)
		return
	}

	fmt.Println("Connected to WebSocket client")
	for {
		select {
		case podEvent := <-podWatchInterface.ResultChan():
			logEvent(podEvent, logFile)

		case <-time.After(30 * time.Second):
			fmt.Println("Sending a heartbeat message to the WebSocket client")
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				fmt.Printf("Error sending heartbeat: %v\n", err)
				return
			}
		}
	}
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

	// Create a log file to store events
	logFilePath := "events.log"
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		return
	}
	defer logFile.Close()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(w, r, clientset, logFile)
	})

	// Start watching Deployments in the background
	go watchDeployments(clientset, logFile)

	fmt.Println("WebSocket server started on :7008")
	if err := http.ListenAndServe(":7008", nil); err != nil {
		fmt.Printf("ListenAndServe error: %v\n", err)
	}
}
