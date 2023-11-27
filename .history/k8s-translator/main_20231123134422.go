package main

import (
	"context"
	"log"
	"net/http"
	"path/filepath"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	clients   = make(map[*websocket.Conn]bool)
	broadcast = make(chan string)
	upgrader  = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func main() {
	config, err := getConfig()
	if err != nil {
		log.Fatal("Failed to get Kubernetes config:", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal("Failed to create Kubernetes client:", err)
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	setupPodInformer(factory, clientset)
	setupServiceInformer(factory, clientset)

	go handleMessages()

	factory.Start(context.Background().Done())

	http.HandleFunc("/ws", handleConnections)
	log.Println("Starting WebSocket server on :8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Failed to start WebSocket server:", err)
	}
}

func getConfig() (*rest.Config, error) {
	if home := homedir.HomeDir(); home != "" {
		kubeconfig := filepath.Join(home, ".kube", "config")
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

func setupPodInformer(factory informers.SharedInformerFactory, clientset *kubernetes.Clientset) {
	podInformer := factory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			logPodEvent("added", pod)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			newPod := newObj.(*corev1.Pod)
			logPodEvent("updated", newPod)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			logPodEvent("deleted", pod)
		},
	})
}

func setupServiceInformer(factory informers.SharedInformerFactory, clientset *kubernetes.Clientset) {
	serviceInformer := factory.Core().V1().Services().Informer()
	serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			service := obj.(*corev1.Service)
			logServiceEvent("added", service)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			newService := newObj.(*corev1.Service)
			logServiceEvent("updated", newService)
		},
		DeleteFunc: func(obj interface{}) {
			service := obj.(*corev1.Service)
			logServiceEvent("deleted", service)
		},
	})
}

func logPodEvent(eventType string, pod *corev1.Pod) {
	message := "Pod " + eventType + ": " + pod.Namespace + "/" + pod.Name
	log.Println(message)
	broadcast <- message
}

func logServiceEvent(eventType string, service *corev1.Service) {
	message := "Service " + eventType + ": " + service.Namespace + "/" + service.Name
	log.Println(message)
	broadcast <- message
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer ws.Close()

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
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("WebSocket write error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
