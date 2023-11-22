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

type customResponseWriter struct {
	http.ResponseWriter
	headersWritten bool
}

func (w *customResponseWriter) WriteHeader(statusCode int) {
	w.headersWritten = true
	w.ResponseWriter.WriteHeader(statusCode)
}

func serveWs(w http.ResponseWriter, r *http.Request, clientset *kubernetes.Clientset) {
	customWriter := &customResponseWriter{ResponseWriter: w}

	conn, err := upgrader.Upgrade(customWriter, r, nil)
	if err != nil {
		fmt.Println("Error upgrading to WebSocket:", err)
		if !customWriter.headersWritten {
			http.Error(customWriter, "Could not open websocket connection", http.StatusBadRequest)
		}
		return
	}
	defer conn.Close()

	watchInterface, err := clientset.CoreV1().Events("").Watch(context.Background(), metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("involvedObject.kind", "Pod").String(),
	})
	if err != nil {
		fmt.Println("Error watching events:", err)
		return
	}

	fmt.Println("Watching for pod events...")
	for event := range watchInterface.ResultChan() {
		jsonEvent, err := json.Marshal(event)
		if err != nil {
			fmt.Println("Error encoding event:", err)
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, jsonEvent); err != nil {
			fmt.Println("Error writing message:", err)
			break
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
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(w, r, clientset)
	})

	fmt.Println("WebSocket server started on :8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
