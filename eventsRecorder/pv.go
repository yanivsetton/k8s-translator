package main

import (
	"context"
	"encoding/json"
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

	watchPVAndPVCEvents(clientset)
}

func watchPVAndPVCEvents(clientset *kubernetes.Clientset) {
	pvSelector := fields.OneTermEqualSelector("involvedObject.kind", "PersistentVolume").String()
	pvcSelector := fields.OneTermEqualSelector("involvedObject.kind", "PersistentVolumeClaim").String()
	combinedSelector := pvSelector + "," + pvcSelector

	watchInterface, err := clientset.CoreV1().Events("").Watch(context.Background(), metav1.ListOptions{
		FieldSelector: combinedSelector,
	})
	if err != nil {
		panic(err.Error())
	}

	fmt.Println("Watching for Persistent Volume and Persistent Volume Claim events...")
	for event := range watchInterface.ResultChan() {
		jsonEvent, err := json.Marshal(event)
		if err != nil {
			fmt.Printf("Error encoding event: %s\n", err)
			continue
		}

		fmt.Println(string(jsonEvent))
	}
}
