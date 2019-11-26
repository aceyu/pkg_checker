package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

var (
	configmap_name = "package_checker_config"
	configmap_key  = "config"
)

type config struct {
	Namespaces []string `json:"namespaces"`
	Path       []string `json:"path"`
}

func getConfigmap(clientset *kubernetes.Clientset) (config, error) {
	var config config
	cmClient := clientset.CoreV1().ConfigMaps(apiv1.NamespaceDefault)
	cm, err := cmClient.Get(configmap_name, metav1.GetOptions{})
	if err != nil {
		return config, err
	}
	value, ok := cm.Data[configmap_key]
	if !ok {
		return config, errors.New("无法获取配置文件")
	}
	err = json.Unmarshal([]byte(value), &config)
	return config, err
}

var clientConfig *rest.Config

func executeRemoteCommand(clientset *kubernetes.Clientset, pod *v1.Pod, command string) (string, string, error) {
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	request := clientset.RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Command: []string{command},
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
			TTY:     true,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", request.URL())
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: buf,
		Stderr: errBuf,
	})
	if err != nil {
		return "", "", errors.New(fmt.Sprintf("Failed executing command %s on %v/%v, err=[%v]", command, pod.Namespace, pod.Name, err))
	}

	return buf.String(), errBuf.String(), nil
}

func main() {
	kubeconfig := "/Users/Tibbers/.kube/config"
	var err error
	clientConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		panic(err)
	}

	// pcconfig, err := getConfigmap(clientset)
	// if err != nil {
	// 	panic(err)
	// }
	pcconfig := &config{
		Namespaces: []string{"kube-system"},
	}

	deployNameMapper := make(map[string]map[string]bool)
	for _, ns := range pcconfig.Namespaces {
		deployNames := make(map[string]bool, 0)
		deploymentsClient := clientset.AppsV1().Deployments(ns)
		deployments, err := deploymentsClient.List(metav1.ListOptions{})
		if err != nil {
			panic(err)
		}
		for _, deployment := range deployments.Items {
			deployNames[deployment.Name] = false
		}
		deployNameMapper[ns] = deployNames
	}

	avaliablePods := make([]v1.Pod, 0)
	for ns, namemap := range deployNameMapper {
		podsClient := clientset.CoreV1().Pods(ns)
		pods, err := podsClient.List(metav1.ListOptions{})
		if err != nil {
			panic(err)
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase != v1.PodRunning {
				continue
			}
			name := pod.Name
			name = name[:strings.LastIndex(name, "-")]
			name = name[:strings.LastIndex(name, "-")]
			if e, ok := namemap[name]; ok {
				if !e {
					avaliablePods = append(avaliablePods, pod)
					namemap[name] = true
				}
			}
		}
	}

	for _, pod := range avaliablePods {
		stdout, stderr, err := executeRemoteCommand(clientset, &pod, "ls")
		if err != nil {
			panic(err)
		}
		fmt.Println(stdout, stderr)
	}
}
