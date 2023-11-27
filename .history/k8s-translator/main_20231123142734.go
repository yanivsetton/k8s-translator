package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/homedir"
)

type K8sEvent struct {
	EventType string      `json:"eventType"`
	Resource  string      `json:"resource"`
	Namespace string      `json:"namespace"`
	Name      string      `json:"name"`
	Details   interface{} `json:"details,omitempty"`
}

var (
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan K8sEvent)
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func main() {
	config, err := getConfig()
	if err != nil {
		log.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	setupPodInformer(factory)
	setupServiceInformer(factory)
	setupDeploymentInformer(factory)

	go handleMessages()

	factory.Start(context.Background().Done())

	http.HandleFunc("/ws", handleConnections)
	log.Println("Starting WebSocket server on :7008")
	err = http.ListenAndServe(":7008", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func getConfig() (*rest.Config, error) {
	if home := homedir.HomeDir(); home != "" {
		kubeconfig := filepath.Join(home, ".kube", "config")
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

func setupPodInformer(factory informers.SharedInformerFactory) {
	podInformer := factory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		// Implement AddFunc, UpdateFunc, and DeleteFunc for Pods here
	})
}

func setupServiceInformer(factory informers.SharedInformerFactory) {
	serviceInformer := factory.Core().V1().Services().Informer()
	serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		// Implement AddFunc, UpdateFunc, and DeleteFunc for Services here
	})
}

func setupDeploymentInformer(factory informers.SharedInformerFactory) {
	deploymentInformer := factory.Apps().V1().Deployments().Informer()
	deploymentInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		// Implement AddFunc, UpdateFunc, and DeleteFunc for Deployments here
	})
}

func broadcastEvent(resource, eventType, namespace, name string, details interface{}) {
	event := K8sEvent{
		EventType: eventType,
		Resource:  resource,
		Namespace: namespace,
		Name:      name,
		Details:   details,
	}
	broadcast <- event
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	clients[conn] = true

	for {
		select {
		case event := <-broadcast:
			eventJSON, err := json.Marshal(event)
			if err != nil {
				log.Printf("Error marshalling event: %v", err)
				continue
			}
			err = conn.WriteMessage(websocket.TextMessage, eventJSON)
			if err != nil {
				log.Printf("Error sending message: %v", err)
				conn.Close()
				delete(clients, conn)
				return
			}
		}
	}
}

func handleMessages() {
	for {
		event := <-broadcast
		fmt.Println("Received event:", event)
		// Implement handling of events here
	}
}
