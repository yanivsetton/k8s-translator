package main

import (
	"context"
	"fmt"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	go watchPods(clientset)
	go watchDeployments(clientset)
	watchServices(clientset)
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
		} else {
			fmt.Printf("Pod Event Type: %s, Pod Name: %s\n", event.Type, pod.GetName())
		}
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
		} else {
			fmt.Printf("Deployment Event Type: %s, Deployment Name: %s\n", event.Type, deployment.GetName())
		}
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
		} else {
			fmt.Printf("Service Event Type: %s, Service Name: %s\n", event.Type, service.GetName())
		}
	}
}
