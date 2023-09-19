# kubeconfig-generator
사용자가 생성한 SA와 Secret을 바탕으로 YAML 또는 JSON 형식의 kubeconfig 파일을 생성합니다.

# Prerequisites
- go >= 1.20
- kubectl
- kubernetes cluster 와 접근 권한이 있는 kubeconfig

# Usage
```zsh
# zsh
git clone https://github.com/Seo-yul/kubeconfig-generator.git -o kubeconfig-generator
cd kubeconfig-generator
go mod tidy
go build -o kubectl-make-kubeconfig
cp kubectl-make-kubeconfig /usr/local/bin

kubectl make kubeconfig --service-account <service-account-name> [--namespace <namespace>]

# output: <service-account-name-[index]>.kubeconfig
```

# Help
```zsh
# zsh
kubectl make kubeconfig --help

  -n string
    	Service Account Namespace (default "default")
  -namespace string
    	Service Account Namespace (default "default")
  -o string
    	Output Type (default "yaml")
  -output string
    	Output Type (default "yaml")
  -sa string
    	Service Account Name
  -service-account string
    	Service Account Name
```