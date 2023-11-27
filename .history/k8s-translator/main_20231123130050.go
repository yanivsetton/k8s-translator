package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type K8sEvent struct {
	Type      string `json:"type"`
	Object    string `json:"object"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Source    string `json:"source"`
}

var (
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan K8sEvent)
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow connections from any origin
		},
	}
)

func main() {
	var config *rest.Config
	var err error

	if home := homedir.HomeDir(); home != "" {
		kubeconfig := filepath.Join(home, ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		log.Fatal("Error getting Kubernetes config:", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal("Error creating Kubernetes client:", err)
	}

	go watchEvents(clientset)
	go handleMessages()

	http.HandleFunc("/ws", handleConnections)
	log.Println("Starting WebSocket server on :7008")
	err = http.ListenAndServe(":7008", nil)
	if err != nil {
		log.Fatal("Error starting WebSocket server:", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer ws.Close()

	log.Println("WebSocket client connected")
	clients[ws] = true

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			delete(clients, ws)
			break
		}
	}
}

func handleMessages() {
	for {
		event := <-broadcast
		for client := range clients {
			err := client.WriteJSON(event)
			if err != nil {
				log.Printf("WebSocket write error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func watchEvents(clientset *kubernetes.Clientset) {
	log.Println("Starting to watch Kubernetes events...")
	watch, err := clientset.CoreV1().Events("").Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Fatal("Failed to start watching events:", err)
	}

	for event := range watch.ResultChan() {
		if e, ok := event.Object.(*corev1.Event); ok {
			k8sEvent := K8sEvent{
				Type:      string(event.Type),
				Object:    e.InvolvedObject.Kind,
				Namespace: e.InvolvedObject.Namespace,
				Name:      e.InvolvedObject.Name,
				Reason:    e.Reason,
				Message:   e.Message,
				Source:    e.Source.Component,
			}
			// Log event to console
			logEvent(k8sEvent)
			// Broadcast event to WebSocket clients
			broadcast <- k8sEvent
		}
	}
}

func logEvent(event K8sEvent) {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("Error marshalling event: %v", err)
		return
	}
	log.Printf("Event received: %s", string(eventJSON))
}
