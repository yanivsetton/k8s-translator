package deployments

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ListAndWatch(clientset *kubernetes.Clientset) {
	listDeployments(clientset)
	watchDeployments(clientset)
}

func listDeployments(clientset *kubernetes.Clientset) {
	deployments, err := clientset.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error listing deployments: %v\n", err)
		return
	}
	fmt.Println("Current deployments in cluster:")
	for _, deploy := range deployments.Items {
		fmt.Printf(" - %s\n", deploy.GetName())
	}
}

func watchDeployments(clientset *kubernetes.Clientset) {
	watchInterface, err := clientset.AppsV1().Deployments("").Watch(context.Background(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error watching deployments: %v\n", err)
		return
	}
	fmt.Println("Watching for deployment changes...")
	for event := range watchInterface.ResultChan() {
		deployment, ok := event.Object.(*appsv1.Deployment)
		if !ok {
			fmt.Printf("Unexpected type\n")
			continue
		}
		fmt.Printf("Deployment Event Type: %s, Deployment Name: %s\n", event.Type, deployment.GetName())
	}
}
