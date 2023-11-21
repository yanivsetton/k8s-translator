package pods

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ListAndWatch(clientset *kubernetes.Clientset) {
	listPods(clientset)
	watchPods(clientset)
}

func listPods(clientset *kubernetes.Clientset) {
	pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing pods: %v\n", err)
		return
	}
	fmt.Println("Current pods in cluster:")
	for _, pod := range pods.Items {
		fmt.Printf(" - %s\n", pod.GetName())
	}
}

func watchPods(clientset *kubernetes.Clientset) {
	watchInterface, err := clientset.CoreV1().Pods("").Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error watching pods: %v\n", err)
		return
	}
	fmt.Println("Watching for pod changes...")
	for event := range watchInterface.ResultChan() {
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			fmt.Printf("Unexpected type\n")
			continue
		}
		fmt.Printf("Pod Event Type: %s, Pod Name: %s\n", event.Type, pod.GetName())
	}
}
