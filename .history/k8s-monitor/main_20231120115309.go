package main

import (
	"context"
	"fmt"
	"path/filepath"

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

	getPods(clientset)
	getDeployments(clientset)
	getServices(clientset)
}

func getPods(clientset *kubernetes.Clientset) {
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error getting pods: %v\n", err)
		return
	}
	fmt.Println("Listing pods in cluster:")
	for _, pod := range pods.Items {
		fmt.Printf(" - %s\n", pod.GetName())
	}
}

func getDeployments(clientset *kubernetes.Clientset) {
	deployments, err := clientset.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error getting deployments: %v\n", err)
		return
	}
	fmt.Println("Listing deployments in cluster:")
	for _, deploy := range deployments.Items {
		replicas := deploy.Spec.Replicas
		if replicas == nil {
			fmt.Printf(" - %s with replicas: not set\n", deploy.GetName())
		} else {
			fmt.Printf(" - %s with replicas: %d\n", deploy.GetName(), *replicas)
		}
	}
}

func getServices(clientset *kubernetes.Clientset) {
	services, err := clientset.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Error getting services: %v\n", err)
		return
	}
	fmt.Println("Listing services in cluster:")
	for _, svc := range services.Items {
		fmt.Printf(" - %s\n", svc.GetName())
	}
}
