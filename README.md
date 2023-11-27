![Build Status](https://github.com/yanivsetton/k8s-translator/workflows/CI/CD/badge.svg)

# k8s-translator
# Kubernetes Event WebSocket Streamer

This Go script provides a simple WebSocket server that streams Kubernetes events in real-time. It watches for events in a Kubernetes cluster and sends them to connected WebSocket clients. The script is designed to be used in conjunction with Kubernetes clusters, making it useful for monitoring and debugging Kubernetes workloads.

## Prerequisites

Before running this script, make sure you have the following prerequisites installed and configured:

1. **Go**: Ensure you have Go installed on your system. You can download and install Go from the [official website](https://golang.org/dl/).

2. **Kubernetes Configuration**: You need access to a Kubernetes cluster, and your `kubectl` command should be configured to connect to the cluster. If running outside a Kubernetes cluster, ensure you have a valid `kubeconfig` file.

3. **AWS CLI (Optional)**: If your Kubernetes cluster is on AWS EKS and your application relies on AWS credentials, you can install the AWS CLI and configure it with your AWS credentials. Ensure the AWS CLI is correctly configured in the environment where you run the script.

## Usage

1. Clone this repository:

   ```bash
   git clone https://github.com/yanivsetton/k8s-translator.git
   cd k8s-translator
   go build -o translator
   ./translator
   ```
