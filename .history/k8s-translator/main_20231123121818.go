package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan interface{})
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func main() {
	// Set up Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal("Failed to get cluster config:", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal("Failed to create clientset:", err)
	}

	go watchEvents(clientset)
	go handleMessages()

	// Set up WebSocket server
	http.HandleFunc("/ws", handleConnections)
	log.Println("Starting WebSocket server on :8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe error:", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	clients[ws] = true

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Error: %v", err)
			delete(clients, ws)
			break
		}
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("Error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}

func watchEvents(clientset *kubernetes.Clientset) {
	watch, err := clientset.CoreV1().Events("").Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		log.Fatal("Failed to start watching events:", err)
	}

	for event := range watch.ResultChan() {
		fmt.Println("Received event:", event)
		broadcast <- event
	}
}
