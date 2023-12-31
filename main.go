package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"strconv"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"os"
	"os/user"
)

func main() {
	// 서비스 어카운트 이름을 입력받기 위한 플래그 정의
	var saName string // 필수값
	var saNamespace string
	var outputType string

	flag.StringVar(&saName, "sa", "", "Service Account Name")
	flag.StringVar(&saName, "service-account", "", "Service Account Name")
	flag.StringVar(&saNamespace, "n", "", "Service Account Namespace")
	flag.StringVar(&saNamespace, "namespace", "", "Service Account Namespace")
	flag.StringVar(&outputType, "o", "yaml", "Output Type")
	flag.StringVar(&outputType, "output", "yaml", "Output Type")

	flag.Parse()

	if saName == "" {
		fmt.Println("사용법: kubectl make kubeconfig --service-account=<ServiceAccountName>")
		fmt.Println("       kubectl make kubeconfig --sa=<ServiceAccountName>")

		os.Exit(1)
	}

	if outputType != "yaml" && outputType != "json" {
		fmt.Println("output 타입은 yaml 또는 json 이어야 합니다.")
		os.Exit(1)
	}

	isKubeconfigEnv := false
	// 환경 변수가 설정되어 있으면 kubeconfig 파일을 사용하지 않음
	if os.Getenv("KUBECONFIG") != "" {
		fmt.Println("KUBECONFIG 환경 변수가 설정되어 있습니다. 기본 kubeconfig 파일을 사용하지 않습니다.")
		isKubeconfigEnv = true

	}

	var kubeconfigPath *string

	// 기본 kubeconfig
	if !isKubeconfigEnv {
		currentUser, err := user.Current()
		if err != nil {
			fmt.Printf("현재 유저를 가져오는데 실패: %v\n", err)
			os.Exit(1)
		}

		// 홈 디렉토리에서 kubeconfig 파일 경로 가져오기
		homeDir := currentUser.HomeDir
		realPath := homeDir + "/.kube/config"
		kubeconfigPath = &realPath
		//fmt.Printf("Kubeconfig Path: %s\n", *kubeconfigPath)
	}

	// 사용자 정의 kubeconfig
	if isKubeconfigEnv {
		//fmt.Printf("Kubeconfig Path: %s\n", os.Getenv("KUBECONFIG"))
		envPath := os.Getenv("KUBECONFIG")
		kubeconfigPath = &envPath
	}

	// kubeconfig 파일을 사용하여 Kubernetes 클러스터에 연결하는 설정 생성
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfigPath)
	if err != nil {
		fmt.Printf("kubeconfig 파일을 사용하여 Kubernetes 클러스터에 연결하는 설정 생성 에러: %v\n", err)
		os.Exit(1)
	}

	// 클라이언트 셋업
	// 네임스페이스 "" 인 경우 default 처럼 동작함
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("클라이언트 셋업 에러: %v\n", err)
		os.Exit(1)
	}

	// kubeconfig 파일에 설정된 namespace를 clientcmdapi를 통해서 사용
	// kubeconfig 파일에 설정된 namespace가 없는 경우 "" 빈 문자열이 리턴됨
	userConfig, err := clientcmd.LoadFromFile(*kubeconfigPath)
	if err != nil {
		fmt.Printf("kubeconfig 파일 로드 에러: %v\n", err)
		os.Exit(1)
	}
	userConfigNamespace := userConfig.Contexts[userConfig.CurrentContext].Namespace
	if saNamespace == "" && userConfigNamespace != "" {
		saNamespace = userConfigNamespace
	}

	if saNamespace == "" {
		saNamespace = "default"
	}

	sa, err := clientset.CoreV1().ServiceAccounts(saNamespace).Get(context.TODO(), saName, metav1.GetOptions{})
	if err != nil {
		fmt.Printf("네임스페이스 %s 에서 서비스 어카운트를 찾을 수 없습니다.: %s\n", saNamespace, saName)
		os.Exit(1)
	}

	//fmt.Printf("서비스 어카운트를 찾았습니다.: %s\n", saName)
	secretNameList := sa.Secrets

	if len(secretNameList) == 0 {
		fmt.Printf("서비스 어카운트에 시크릿이 없습니다. 시크릿을 생성하시겠습니까?(Y / other) \n")
		var answer string
		fmt.Scanln(&answer)

		if answer != "y" && answer != "Y" {
			os.Exit(1)
		}

		// 시크릿 생성
		serviceAccountSecret(saName, saNamespace, clientset)
		sa, _ = clientset.CoreV1().ServiceAccounts(saNamespace).Get(context.TODO(), saName, metav1.GetOptions{})
		secretNameList = sa.Secrets
	}

	for i, secretName := range secretNameList {
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
		cnt := ""
		if i > 0 {
			cnt = "-" + strconv.Itoa(i+1)
		}
		destinationDir := currentDir + "/" + saName + cnt
		makeKubeconfigFile(newConfig, destinationDir, outputType)
		fmt.Printf("시크릿 명: %s\n", secretName.Name)
	}
}

func serviceAccountSecret(saName, saNamesapce string, clientset *kubernetes.Clientset) {

	// 랜덤 문자열 생성
	randString := generateRandomString(5)

	// 시크릿 생성
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName + "-secret-" + randString,
			Namespace: saNamesapce,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": saName,
			},
		},
		Type: "kubernetes.io/service-account-token",
	}

	_, err := clientset.CoreV1().Secrets(saNamesapce).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("시크릿 생성 에러: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("시크릿 생성.")

	// 서비스 어카운트에 시크릿 연결
	serviceAccount := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: saNamesapce,
		},
		Secrets: []v1.ObjectReference{
			{
				Name: secret.Name,
			},
		},
	}

	_, err = clientset.CoreV1().ServiceAccounts(saNamesapce).Update(context.Background(), serviceAccount, metav1.UpdateOptions{})
}

func makeKubeconfigFile(config *rest.Config, destinationDir string, outputType string) {

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

	if outputType == "json" {
		// kubeconfig 데이터 json 변환
		jsonData, err := json.MarshalIndent(newConfig, "", "  ")
		if err != nil {
			fmt.Printf("json 변환 에러: %v\n", err)
			os.Exit(1)
		}

		// json 데이터 파일 저장
		err = os.WriteFile(destinationDir, jsonData, 0644)
		if err != nil {
			fmt.Printf("json 파일 쓰기 에러: %v\n", err)
			os.Exit(1)
		}
	}

	if outputType == "yaml" {
		// yaml 데이터 파일 저장
		err := clientcmd.WriteToFile(*newConfig, destinationDir)
		if err != nil {
			fmt.Printf("yaml 파일 쓰기 에러: %v\n", err)
			os.Exit(1)
		}
	}

	//fmt.Printf("kubernetes API Server: %s\n", config.Host)
	fmt.Printf("서비스 어카운트 명: %s\n", myUserName)
}

func generateRandomString(length int) string {
	rand.Seed(time.Now().UnixNano())

	const charset = "abcdefghijklmnopqrstuvwxyz" // 사용할 문자셋
	result := make([]byte, length)

	for i := 0; i < length; i++ {
		result[i] = charset[rand.Intn(len(charset))]
	}

	return string(result)
}
