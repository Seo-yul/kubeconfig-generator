# kubeconfig-generator
사용자가 생성한 SA와 Secret을 바탕으로 kubeconfig를 생성합니다.

# Prerequisites
- kubectl
- go >= 1.20
- kubernetes cluster
- kubernetes cluster에 접근할 수 있는 kubeconfig

# Usage
```bash
git clone https://github.com/Seo-yul/kubeconfig-generator.git -o kubeconfig-generator
cd kubeconfig-generator
go mod tidy
go build -o kubectl-make-kubeconfig
cp kubectl-make-kubeconfig /usr/local/bin

kubectl make kubeconfig --help
kubectl make kubeconfig --service-account <service-account-name> [--namespace <namespace>]

# output: <service-account-name>.kubeconfig
```