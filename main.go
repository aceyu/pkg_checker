package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/exec"
	"os"
	"reflect"
	"strings"

	"k8s.io/api/core/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

var (
	configmap_name = "package-checker-config"
	configmap_key  = "config"
	env_name       = "PRODUCTION_IP_MAPPING"
	Commonds       = []string{"ls -l /usr/local/tomcat/webapps/", "cat ./LS_INFO"}
)

type config struct {
	Namespaces []string `json:"namespaces"`
}

func getConfigmap(clientset *kubernetes.Clientset) config {
	var config config
	cmClient := clientset.CoreV1().ConfigMaps(apiv1.NamespaceDefault)
	cm, err := cmClient.Get(configmap_name, metav1.GetOptions{})
	if err != nil {
		fmt.Println("无法从容器云获取配置")
		panic(err)
	}
	value, ok := cm.Data[configmap_key]
	if !ok {
		panic(errors.New("无法获取配置文件"))
	}
	err = json.Unmarshal([]byte(value), &config)
	if err != nil {
		fmt.Println("无法解析配置文件")
		panic(err)
	}
	return config
}

var clientConfig *rest.Config

func executeRemoteCommand(clientset *kubernetes.Clientset, pod *v1.Pod, containerName string, command []string) (string, string, error) {
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	request := clientset.CoreV1().RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
			Container: containerName,
		}, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(clientConfig, "POST", request.URL())
	if err != nil {
		return "", "", err
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: buf,
		Stderr: errBuf,
	})
	if err != nil {
		return "", "", err
	}

	return buf.String(), errBuf.String(), nil
}

func main() {
	var err error

	c := `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN5RENDQWJDZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREFWTVJNd0VRWURWUVFERXdwcmRXSmwKY201bGRHVnpNQjRYRFRFNE1USXhNakEzTkRreU5sb1hEVEk0TVRJd09UQTNORGt5Tmxvd0ZURVRNQkVHQTFVRQpBeE1LYTNWaVpYSnVaWFJsY3pDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUJBTlJSCnNxeDllOEpjU29JU1lFUDBiMWhXSFRyVE00a3puK2NJVi9VR2I1UVBiTXJEZW9iTXNsK0NvU2lJNWQzVUE2bzUKbWJ5WG95cWhIRzFCTU5OaE4wWi9Ua1lNWUVRRE55K0ZPb3MvYktjTFIrREVIK29hZTRXQkRWNXJIMGFNdGEvSQpwSFVIendEeVpNWUpIUG5zOTFFY3Zjc0dqekF4MEVhbUtXL3MreFZINStmYXhmMDJOc04rb2JuZUJ0cFNoSXQzCjVxMHpZdG1Ra3Y5dHF4V1NYR2lwS2tHWHV4OVBXaTVUd0N5OW8wZDFUYk4waC9qZ3dOdityV2dmVWIwY1ROS3kKRTlXcmdWZW9aenE1VDN3cEY4NmRWb0xZNk11N2RaVDVkS1p4SFk4YzJQbjBqZDhJZHZJZTBqdnpERkQ4RTZtNwpRVVVrZDdOYkNraVlLbkFqOG9NQ0F3RUFBYU1qTUNFd0RnWURWUjBQQVFIL0JBUURBZ0trTUE4R0ExVWRFd0VCCi93UUZNQU1CQWY4d0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dFQkFNUnMyTHhNM1p6M2pQdk5ZMHJzYjVVazcvaDUKQnZOTERPcG1SOHlLVFVzVmppMmYvWmpaNnkwRGJwOEtvNEpHa0FuaHdmUUcxY05JOTAvNUd3SlV4eVp4MGJjZQo0WUVVcGhFeUVNanFvN0doZ09RdTFZc3pubnBIRitRcmZTaEUyRUpxY216UDVacytuWGZjWG9ORDN4b3lKU0RoCmpEQXFqSjVOejlEbHozUlJFU3lNTlk1YjEwM2ZveStZZ3hmQW8zZUludld3ZHBLVU8rdkNBNVdzL2o3UDhQUGgKMGxyZWkzL24yUngvWFB2MXV6enBWRzVtVmhvVHhhVzE1SEc4OXVGUUdqdVZ2NEtMWGFoN2Q3dlpaWHhzQmRXZgpBbnFQV09EZTVMVUh3QlBZdVY4TmxwSk5RU1FyellBQ0NhVWxvTk00dTMzREV6dDNueUN0ZzN4NDMxdz0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
    server: https://10.2.10.29:6443
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: kubernetes-admin
  name: kubernetes-admin@kubernetes
- context:
    cluster: kubernetes
    user: jenkins
  name: jenkins@kubernetes
current-context: kubernetes-admin@kubernetes
kind: Config
preferences: {}
users:
- name: kubernetes-admin
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUROVENDQWgyZ0F3SUJBZ0lVZW04cVBJMWppc2VOZkMvN3h2dkxkR0NnMmx3d0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZURVRNQkVHQTFVRUF4TUthM1ZpWlhKdVpYUmxjekFlRncweE9URXdNakl3TWpVNU1EQmFGdzB5T1RFdwpNVGt3TWpVNU1EQmFNRFF4RnpBVkJnTlZCQW9URG5ONWMzUmxiVHB0WVhOMFpYSnpNUmt3RndZRFZRUURFeEJyCmRXSmxjbTVsZEdWekxXRmtiV2x1TUlJQklqQU5CZ2txaGtpRzl3MEJBUUVGQUFPQ0FROEFNSUlCQ2dLQ0FRRUEKNXU3Smx0NG5zNWNOeHN0ZEl4RDQrNnpmMVg4QTZyWVlyNmdCOHFvbnZnUHdOSVFoNW8ybnhIT3Fxa1RLKzVwQwpISitGc2dRL1BSOC9ZanBqRUFnMVBkSktULzNMcFppcjNkMGswblpVckpxd0VPY05MWkFUaTR2dWxhOE5vWGJvCkVjU3F1WWpyZnFoTUJ1RDl1dWc2MnE5cGh3c0F6LzNXV2l1cmlQbm1mUWxHbmVnNkFnWXRncDdZZldJZWJnemIKTHVPTkp5UWk3VFhZVWxockttNFVFd1JHMm9KVEJiRTJaZzlCeWwwSUl1WERmUU9ObTI1ZE5zamExMGxHV1VNRQoyVnJkdVAveHJRN1FhNmcxSTNEcDVVaW15QXZSMVVBK2FUUGRSd0UyTk5mYytwVVhvdFl6V3FhVDBMeUEwcGJvCitZWnBOTVNBeHVmTEVoYXdtYk1JVHdJREFRQUJvMTR3WERBT0JnTlZIUThCQWY4RUJBTUNCYUF3SFFZRFZSMGwKQkJZd0ZBWUlLd1lCQlFVSEF3RUdDQ3NHQVFVRkJ3TUNNQXdHQTFVZEV3RUIvd1FDTUFBd0hRWURWUjBPQkJZRQpGQzFjTUVEVVovY092dnJrOTA5UkJsTFowalZMTUEwR0NTcUdTSWIzRFFFQkN3VUFBNElCQVFCblFvRllDcDJYCnFDdUpZdkdlSTdEcDB1elBtZkJPaFBhamtxK0tQZ3JQZUM4b0NQTXpVek5HbG83VzVDRld5eDFBNjhqZjFmU1UKYlFnRC80Q2M1Y3RBeWVvb1dNMk1uWjRnQ09nNEw2dUV6REQrWFdFaWlFenBLVldZQmJjUHlJeElxQUVremtlTApLWFFMQ3VTQk1ldXkrd01sZThsVXNHWWRQdnMzcHhJdTNkb1k3bkdrWE9jbzBEYkhvL2RoQmhSUGpHNlpoNGNhCmM0YldEWFhNYytNUDN6YlNtYWM1YU9adTE5dmljenN2ejBxaXZjZlppQzR5V0VDVEcyZjJFOWNWMG94ZmNFQ0kKWkNYdXJId1RaMndsZGIrK1dvWDZFOXZMc2UvWnZYc1FrVU40K3IrRTRhQStTUWM2ZG9LWjU2VjA0YW5sSnNhQgpmMHZDSjQ0NXhMTmcKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
    client-key-data: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcEFJQkFBS0NBUUVBNXU3Smx0NG5zNWNOeHN0ZEl4RDQrNnpmMVg4QTZyWVlyNmdCOHFvbnZnUHdOSVFoCjVvMm54SE9xcWtUSys1cENISitGc2dRL1BSOC9ZanBqRUFnMVBkSktULzNMcFppcjNkMGswblpVckpxd0VPY04KTFpBVGk0dnVsYThOb1hib0VjU3F1WWpyZnFoTUJ1RDl1dWc2MnE5cGh3c0F6LzNXV2l1cmlQbm1mUWxHbmVnNgpBZ1l0Z3A3WWZXSWViZ3piTHVPTkp5UWk3VFhZVWxockttNFVFd1JHMm9KVEJiRTJaZzlCeWwwSUl1WERmUU9OCm0yNWROc2phMTBsR1dVTUUyVnJkdVAveHJRN1FhNmcxSTNEcDVVaW15QXZSMVVBK2FUUGRSd0UyTk5mYytwVVgKb3RZeldxYVQwTHlBMHBibytZWnBOTVNBeHVmTEVoYXdtYk1JVHdJREFRQUJBb0lCQUQwTVVSUm1CQjdReHQ2UApzajVyNVRZN0hDMEhWd20xTzg5cjNaLzE1VzJ4QXRZUFBCc0R4WjhFYU5COFFTREVSY2ZsVCtXZ2c4czNzSHphCkxJZjNjNE8xVE5uYW9QUlU2TkpNL01mNmFpWDYrcUp0UWltU1ZlaGxCSnhqVzNvY3dmcTRmOTF1V2JydzZMQkUKMkM2SjU4MFo1QTdFRk9Ibkc3eFlvUThqNlErU1hKVmdZWEEyQi84bkkvOUI2UzJUL1ZMd2pMSllYVkd0RjQwYwpCZTFJS3F0a2dacklUSnR5SW0yZi9xQndhc2Z4cjJ4SzdYZU95MG1RM2pxdUpBRlJhS0d5ajRnK2RGNjZhaTZSCm12Sy9FTTJxQUhZVGFDNFJvTTFCNVJWcm1mWjc3T1RIa1VySDVJZTRaYXJPZEQxWWJxZE54dzM2ZzF4aFBPRDMKd3ZIU0tPRUNnWUVBN1JlNmc2RDRXaVd6d1pYVGt5UEZJUHZFZFdKSENYNHRGc0FZNDh6ZWpMQjBPejNLUmk2Tgo5OHd1NDVNOHN3L0phdDF6aHRqR0tPeDh4RWx4cUFKQXlBbWowNVNFVlRlMHBJcHI1ZUQyNVMvQVd6bVVsM3JzCmFhWGNEWVU4ZTVMcVFyaEVpZ2IrVHhJUlNNalRkSkM5MDFEbG9hMlp5REFWalFGTi9QZVI3eDhDZ1lFQStWbE4KcVY0QXpmdE54MFRiSDVmUEk1MTM5bzJvaTMzZEV1MURDNFUvbm9Cd0ZZMXBmay9HaGwrUUVkaEd2MTdKeVE2SApGeHhRNUtlOWp3Q3RzQnZwZk9qWXdYMTdIVTRySjRrcitWL2FEbk5oc28rcUlPNDJQR3UxaEoxVnJlRVpQd3lRCitoZHFRdkpLVzBnbjh6YmI5MHdYbnNPQlNtT0ozVGZxd3k4bk1ORUNnWUVBaFJ2L1VRczhvNC9yUGRJYU9NK3EKU3Z4T3JnQ0JGV2xMY3l4aVRQS21ONktSZnZrUDZSc1dCWHNURUIySHhKZ21ZdUwxaTAyRTQxRHlNMWx3Zi96VAoxZnJqaVZRbWY1bUl4NkFYTjdaM3B2Q0tOQzA5cVZZUUNMaGZ0UStLaDI1U0t5YzlBNmt0ZWNNUkJTWUs0YlNwCmZrdzZ2K3l4RzkwekhEa1JTZWJNZmMwQ2dZRUFobzJuTjk3dkpqZ1hCNUhqZ00vbHlqMCtNQURQVTc2dW5ua0QKOWVLSXF4cDU0VmQyOXQ5THJOVkNwQzZHTnR5S25RRkc2clN2L2tONktnSGV1Q3JIdTB6WE1zcG90aTZwWU9OSApwSUVSNVR4a0d2d2xmVEd1ZUxwU3NHWktodEx5VWJDUlJ6TjlkdlRTSlNIeDFPL2trVFV4aGMzUUpmbEN1dXBpCnQ4THBMaEVDZ1lCQXhNWDduTk5venZabHhwWHJLb0QrQjZoVXRHV0V4VVd3QmMvMmJ5d1BraWNnYlpYUkZQYlcKU0U4bHpQTnJVeTJiZllFUlQ3eUc2SDRnQXlmZGczQlBlbGlray9TbWR4UXFIalFrS0RSUHhhQVZRclNEZzVDYwpOSUN3TlFhVytNVFREamU3NDRhcTQyWlZlODJ1NnFXMTk0TW9HV0VUc2o0Q1p2Tk9mK2NmYmc9PQotLS0tLUVORCBSU0EgUFJJVkFURSBLRVktLS0tLQo=
- name: jenkins
  user:
    token: eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJwLWNqbHh6LXBpcGVsaW5lIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImplbmtpbnMtdG9rZW4tcGc3eHYiLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC5uYW1lIjoiamVua2lucyIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50LnVpZCI6ImU2ODk3YjMyLWZhMjgtMTFlOS04NzMwLWVjZWJiODg4MWQ4MCIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDpwLWNqbHh6LXBpcGVsaW5lOmplbmtpbnMifQ.j8Rt7IRIAc4yzxGJIroHbpWK032-X8902Sm9raTh6t9NnCu2_mTZa6Tj4OWIlNlKnjnBxsrX7r7vLSArEH-imVUWlvkJNPJi6bhw61Gzh7IccwyL3G4Fu-Bpjfev2-BaD-gJzpn8pWJ1pbdrYXBv4TjeG_gNcXoRVbGggwi_gHX0nUFmS-N6X-SxVmRADN10L8oV8simlq0IOCk6uv5FpkO91SP4TFTvqQox9_2IaXDff8tVpzGsUUqyG08V_AmLulRD1s5na_V8j1Cntt76hTloN3bgalNJpWebVkQ2hv2zIA4xlUiCilwV_YFXkevrTw5aeo83eXig0hK8G2Y9Vw`
	path := os.TempDir() + "be3a6975-75c7-4cbe-b85a-eca5b56fd6b3"
	err = ioutil.WriteFile(path, []byte(c), 0666)
	if err != nil {
		panic(err)
	}
	kubeconfig := path
	//kubeconfig = "/Users/zac/Desktop/k8s/config29"
	clientConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}
	clientset, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		panic(err)
	}

	pcconfig := getConfigmap(clientset)
	if err != nil {
		panic(err)
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
			status := true
			for _, st := range pod.Status.ContainerStatuses {
				if !st.Ready {
					status = false
					break
				}
			}
			if !status {
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
	result := make(map[string]string, 0)
	for _, pod := range avaliablePods {
		if len(pod.Spec.Containers) < 1 {
			continue
		}
		containerName := pod.Spec.Containers[0].Name
		stdout, stderr, err := executeRemoteCommand(clientset, &pod, containerName, []string{"env"})
		if err != nil {
			panic(err)
		}
		if stderr != "" {
			fmt.Println(stderr)
		}
		envs := strings.Split(stdout, "\n")
		ip := "None"
		for _, v := range envs {
			if strings.HasPrefix(v, env_name+"=") {
				ip = v[len(env_name)+1:]
				break
			}
		}
		info := ""
		for _, cmd := range Commonds {
			command := strings.Split(cmd, " ")
			stdout2, stderr2, err := executeRemoteCommand(clientset, &pod, containerName, command)
			if err != nil && reflect.TypeOf(err).String() != "exec.CodeExitError" {
				panic(err)
			}
			if err != nil {
				exitErr := err.(exec.CodeExitError)
				if exitErr.Code == 1 || exitErr.Code == 2 {
					continue
				} else {
					panic(err)
				}
			}
			if stderr2 != "" {
				continue
			} else {
				if strings.HasPrefix(cmd, "ls") {
					a := strings.Split(stdout2, "\n")
					for _, v := range a {
						v = strings.Replace(v, "\r", "", -1)
						if strings.HasSuffix(strings.ToLower(v), ".war") {
							info = v
							break
						}
					}
				} else {
					info = stdout2
				}
				break
			}
		}
		result[ip] = info
	}
	for k, v := range result {
		k = strings.Replace(k, "\r", "", -1)
		k = strings.Replace(k, "\n", "", -1)
		v = strings.Replace(v, "\r", "", -1)
		v = strings.Replace(v, "\n", "", -1)
		fmt.Println(k + " " + v)
	}
}
