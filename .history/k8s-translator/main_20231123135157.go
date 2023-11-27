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
	setupPodInformer(factory)
	setupServiceInformer(factory)

	go handleMessages()
	factory.Start(context.Background().Done())

	http.HandleFunc("/ws", handleConnections)
	log.Println("Starting WebSocket server on :7008")
	err = http.ListenAndServe(":7008", nil)
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

func setupPodInformer(factory informers.SharedInformerFactory) {
	podInformer := factory.Core().V1().Pods().Informer()
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			broadcastEvent("Pod added", pod.Namespace, pod.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod := newObj.(*corev1.Pod)
			broadcastEvent("Pod updated", pod.Namespace, pod.Name)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			broadcastEvent("Pod deleted", pod.Namespace, pod.Name)
		},
	})
}

func setupServiceInformer(factory informers.SharedInformerFactory) {
	serviceInformer := factory.Core().V1().Services().Informer()
	serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			service := obj.(*corev1.Service)
			broadcastEvent("Service added", service.Namespace, service.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			service := newObj.(*corev1.Service)
			broadcastEvent("Service updated", service.Namespace, service.Name)
		},
		DeleteFunc: func(obj interface{}) {
			service := obj.(*corev1.Service)
			broadcastEvent("Service deleted", service.Namespace, service.Name)
		},
	})
}

func broadcastEvent(eventType, namespace, name string) {
	message := eventType + ": " + namespace + "/" + name
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
