package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

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

	watchEvents(clientset)
}

func watchEvents(clientset *kubernetes.Clientset) {
	watcher, err := clientset.CoreV1().Events("").Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: fields.Everything().String(),
	})
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Watching for events...")
	for event := range watcher.ResultChan() {
		fmt.Printf("Event: %v\n", event)
		// Here, you can decide what to do with each event
	}
}
