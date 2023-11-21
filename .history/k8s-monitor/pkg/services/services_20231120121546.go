package services

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ListAndWatch(clientset *kubernetes.Clientset) {
	listServices(clientset)
	watchServices(clientset)
}

func listServices(clientset *kubernetes.Clientset) {
	services, err := clientset.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing services: %v\n", err)
		return
	}
	fmt.Println("Current services in cluster:")
	for _, svc := range services.Items {
		fmt.Printf(" - %s\n", svc.GetName())
	}
}

func watchServices(clientset *kubernetes.Clientset) {
	watchInterface, err := clientset.CoreV1().Services("").Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error watching services: %v\n", err)
		return
	}
	fmt.Println("Watching for service changes...")
	for event := range watchInterface.ResultChan() {
		service, ok := event.Object.(*corev1.Service)
		if !ok {
			fmt.Printf("Unexpected type\n")
			continue
		}
		fmt.Printf("Service Event Type: %s, Service Name: %s\n", event.Type, service.GetName())
	}
}
