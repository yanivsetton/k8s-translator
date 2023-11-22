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
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin
	},
}

func serveWs(w http.ResponseWriter, r *http.Request, clientset *kubernetes.Clientset) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Error upgrading to WebSocket: %v\n", err)
		return
	}
	defer conn.Close()

	replicaSetSelector := fields.OneTermEqualSelector("involvedObject.kind", "ReplicaSet").String()
	statefulSetSelector := fields.OneTermEqualSelector("involvedObject.kind", "StatefulSet").String()
	combinedSelector := replicaSetSelector + "," + statefulSetSelector

	watchInterface, err := clientset.CoreV1().Events("").Watch(context.Background(), metav1.ListOptions{
		FieldSelector: combinedSelector,
	})
	if err != nil {
		fmt.Printf("Error watching ReplicaSet and StatefulSet events: %v\n", err)
		return
	}

	fmt.Println("Connected to WebSocket client for ReplicaSet and StatefulSet events")
	for event := range watchInterface.ResultChan() {
		jsonEvent, err := json.MarshalIndent(event, "", "  ") // Use 2-space indentation
		if err != nil {
			fmt.Printf("Error encoding event: %v\n", err)
			continue
		}

		// Print the event to the server's console
		fmt.Println("ReplicaSet/StatefulSet event received:")
		fmt.Println(string(jsonEvent))

		if err := conn.WriteMessage(websocket.TextMessage, jsonEvent); err != nil {
			fmt.Printf("Error writing message: %v; exiting message loop\n", err)
			break
		}
	}
	fmt.Println("Disconnected from WebSocket client for ReplicaSet and StatefulSet events")
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

	fmt.Println("WebSocket server for ReplicaSet and StatefulSet events started on :7010")
	if err := http.ListenAndServe(":7010", nil); err != nil {
		fmt.Printf("ListenAndServe error: %v\n", err)
	}
}
