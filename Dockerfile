FROM alpine:latest AS security

WORKDIR /sec

# download and install wget
RUN apk update && \
    apk --update --no-cache add wget

RUN wget --no-check-certificate -nv \
        'https://github.com/github/codeql-action/releases/download/codeql-bundle-v2.15.0/codeql-bundle-linux64.tar.gz' \
        -O "codeql.tar.gz" && \
        tar -xzvf "codeql.tar.gz" && \
        rm "codeql.tar.gz"

FROM golangci/golangci-lint AS lint

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

# copy files
COPY . .

# runing lintern
RUN golangci-lint run

# runing tests
RUN go test ./...


FROM alpine:latest as analyze

RUN apk --update add git less openssh && \
    rm -rf /var/lib/apt/lists/* && \
    rm /var/cache/apk/*

RUN adduser -S security -G root
USER security
WORKDIR /home/security

# define folders
RUN mkdir "project" && \
    mkdir "codeql-home" && \
    mkdir "codeql-home/codeql-repo"

# Create a folder for CodeQL
RUN cd "codeql-home/codeql-repo" && \
    git clone https://github.com/github/codeql.git

# copy codeql
COPY --from=security --chown=security:root /sec/codeql/codeql /home/security/codeql-home/codeql

# Add the CodeQL home folder to the PATH
# RUN export PATH=$PATH:/home/security/codeql-home/codeql

# copy project
COPY --from=lint --chown=security:root /usr/src/app /home/security/project

RUN /home/security/codeql-home/codeql --help
RUN /home/security/codeql-home/codeql database create db --language=go

ARG CODEQL_SUITES_PATH=/home/security/codeql-home/codeql-repo/go/ql/src/codeql-suites
ARG RESULTS_FOLDER=codeql-results

# analyzing project
RUN cd /home/security/project && \
    mkdir -p $RESULTS_FOLDER &&  \
    /home/security/codeql-home/codeql database analyze db $CODEQL_SUITES_PATH/go-code-scanning.qls  \
    --format=sarifv2.1.0 \
    --output=$RESULTS_FOLDER/go-code-scanning.sarif && \
    /home/security/codeql-home/codeql database analyze db $CODEQL_SUITES_PATH/go-security-extended.qls \
    --format=sarif-latest \
    --output=$RESULTS_FOLDER/go-security-extended.sarif && \
    /home/security/codeql-home/codeql database analyze db $CODEQL_SUITES_PATH/go-security-and-quality.qls \
    --format=sarif-latest \
    --output=$RESULTS_FOLDER/go-security-and-quality.sarif
