package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan interface{})
var upgrader = websocket.Upgrader{}

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
	watch, err := clientset.CoreV1().Events("").Watch(context.Background(), v1.ListOptions{})
	if err != nil {
		log.Fatal("Failed to start watching events:", err)
	}

	for event := range watch.ResultChan() {
		broadcast <- event
	}
}
