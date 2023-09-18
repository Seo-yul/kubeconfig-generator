package main

import (
	"context"
	"flag"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"os"
	"os/user"
)

func main() {
	// 서비스 어카운트 이름을 입력받기 위한 플래그 정의
	var saName string
	var shortServiceAccountName string
	var saNamespace string
	var shortNamespace string
	flag.StringVar(&saName, "service-account", "", "Service Account Name")
	flag.StringVar(&shortServiceAccountName, "sa", "", "Service Account Name")
	flag.StringVar(&saNamespace, "namespace", "default", "Service Account Namespace")
	flag.StringVar(&shortNamespace, "n", "default", "Service Account Namespace")
	flag.Parse()

	if saName == "" && shortServiceAccountName == "" {
		fmt.Println("사용법: kubectl get-sa-kubeconfig --service-account=<ServiceAccountName>")
		fmt.Println("       kubectl get-sa-kubeconfig --sa=<ServiceAccountName>")

		os.Exit(1)
	}

	if saName == "" && shortServiceAccountName != "" {
		saName = shortServiceAccountName
	}

	if saNamespace == "default" && shortNamespace != "default" {
		saNamespace = shortNamespace
	}

	isKubeconfigEnv := false
	// 환경 변수가 설정되어 있으면 kubeconfig 파일을 사용하지 않음
	if os.Getenv("KUBECONFIG") != "" {
		fmt.Println("KUBECONFIG 환경 변수가 설정되어 있습니다. kubeconfig 파일을 사용하지 않습니다.")
		isKubeconfigEnv = true
	}

	var kubeconfigPath *string
	if !isKubeconfigEnv {
		currentUser, err := user.Current()
		if err != nil {
			fmt.Printf("Error getting current user: %v\n", err)
			os.Exit(1)
		}

		// 홈 디렉토리에서 kubeconfig 파일 경로 가져오기
		homeDir := currentUser.HomeDir
		realPath := homeDir + "/.kube/config"
		kubeconfigPath = &realPath
		fmt.Printf("Kubeconfig Path: %s\n", *kubeconfigPath)
	}

	// kubeconfig 파일을 사용하여 Kubernetes 클러스터에 연결하는 설정 생성
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfigPath)
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("config.Host %s\n", config.Host)

	// 클라이언트 셋업
	// 네임스페이스 "" 인 경우 -A 처럼 동작함
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	sa, err := clientset.CoreV1().ServiceAccounts(saNamespace).Get(context.TODO(), saName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("서비스 어카운트를 찾을 수 없습니다.: %s\n", saName)
		os.Exit(1)
	}
	secretNameList := sa.Secrets
	if len(secretNameList) == 0 {
		fmt.Printf("서비스 어카운트에 시크릿이 없습니다. \n")
		os.Exit(1)
	}
	for _, secretName := range secretNameList {
		secret, err := clientset.CoreV1().Secrets(saNamespace).Get(context.TODO(), secretName.Name, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("시크릿을 찾을 수 없습니다. : %s\n", secretName.Name)
			os.Exit(1)
		}
		if secret.Type == "kubernetes.io/service-account-token" {
			// clientset 으로 가져온 시크릿 데이터는 이미 base64에서 디코딩됨
			if _, ok := secret.Data["token"]; ok {

			} else {
				fmt.Printf("토큰을 읽을 수 없습니다.\n")
				os.Exit(1)
			}
		} else {
			fmt.Printf("시크릿 타입이 kubernetes.io/service-account-token이 아닙니다. : %s\n", secret.Type)
			os.Exit(1)
		}

		newConfig, _ := clientcmd.BuildConfigFromFlags("", *kubeconfigPath)

		newConfig.Username = saName
		newConfig.CertData = secret.Data["ca.crt"]
		newConfig.BearerToken = string(secret.Data["token"])

		currentDir, _ := os.Getwd()
		destinationDir := currentDir + "/" + saName + ".kubeconfig"

		makeKubeconfigFile(newConfig, destinationDir)
	}
}

func makeKubeconfigFile(config *rest.Config, destinationDir string) {

	myClusterName := "my-cluster"
	myContextName := "my-context"
	myUserName := config.Username

	// 새로운 kubeconfig 데이터 구조 생성
	newConfig := &clientcmdapi.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: map[string]*clientcmdapi.Cluster{
			myClusterName: {
				Server:                   config.Host,
				CertificateAuthorityData: config.CertData,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			myContextName: {
				Cluster:   myClusterName,
				Namespace: "default",
				AuthInfo:  myUserName,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			myUserName: {
				Token: config.BearerToken,
			},
		},
		CurrentContext: myContextName,
	}

	// kubeconfig 파일에 저장
	err := clientcmd.WriteToFile(*newConfig, destinationDir)
	if err != nil {
		fmt.Printf("Error writing kubeconfig: %v\n", err)
		os.Exit(1)
	}
}