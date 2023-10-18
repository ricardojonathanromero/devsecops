lint-go: lint-install-go
	@golangci-lint run

lint-install-go:
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

codeQL:
	GH_USER=github \
    GH_REPO=codeql-action \
    GH_BRANCH=main \
    v2.15.0
    wget https://github.com/${GH_USER}/${GH_REPO}/archive/refs/tags/${GH_BRANCH}.tar.gz \
    -O "${GH_REPO}-${GH_BRANCH}.tar.gz" && \
    tar -xzvf ./"${GH_REPO}-${GH_BRANCH}.tar.gz" && \
    rm ./"${GH_REPO}-${GH_BRANCH}.tar.gz"

https://github.com/github/codeql-action/releases/download/codeql-bundle-v2.15.0/codeql-bundle-linux64.tar.gz