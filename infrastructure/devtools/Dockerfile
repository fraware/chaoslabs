# Development tools container with all necessary tools for ChaosLabs development
FROM ubuntu:22.04

LABEL maintainer="ChaosLabs DevOps <devops@chaoslabs.com>"
LABEL description="Development tools container for ChaosLabs"

# Avoid prompts from apt
ENV DEBIAN_FRONTEND=noninteractive

# Install base packages
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    git \
    vim \
    nano \
    htop \
    tree \
    jq \
    yq \
    unzip \
    zip \
    ca-certificates \
    gnupg \
    lsb-release \
    software-properties-common \
    build-essential \
    python3 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Install Go
ENV GO_VERSION=1.21.3
RUN wget -O go.tar.gz "https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
    && tar -C /usr/local -xzf go.tar.gz \
    && rm go.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/go"
ENV PATH="${GOPATH}/bin:${PATH}"

# Install Node.js and npm
ENV NODE_VERSION=18
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_VERSION}.x | bash - \
    && apt-get install -y nodejs

# Install Docker CLI
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg \
    && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null \
    && apt-get update \
    && apt-get install -y docker-ce-cli \
    && rm -rf /var/lib/apt/lists/*

# Install docker-compose
RUN pip3 install docker-compose

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    && install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl \
    && rm kubectl

# Install Helm
RUN curl https://baltocdn.com/helm/signing.asc | gpg --dearmor | tee /usr/share/keyrings/helm.gpg > /dev/null \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/helm.gpg] https://baltocdn.com/helm/stable/debian/ all main" | tee /etc/apt/sources.list.d/helm-stable-debian.list \
    && apt-get update \
    && apt-get install -y helm \
    && rm -rf /var/lib/apt/lists/*

# Install k6 for load testing
RUN gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69 \
    && echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | tee /etc/apt/sources.list.d/k6.list \
    && apt-get update \
    && apt-get install -y k6 \
    && rm -rf /var/lib/apt/lists/*

# Install useful Go tools
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install golang.org/x/tools/cmd/goimports@latest \
    && go install github.com/securecodewarrior/goat@latest \
    && go install golang.org/x/vuln/cmd/govulncheck@latest \
    && go install github.com/air-verse/air@latest

# Install development utilities
RUN npm install -g \
    typescript \
    @typescript-eslint/parser \
    @typescript-eslint/eslint-plugin \
    prettier \
    eslint \
    nodemon \
    concurrently

# Install network debugging tools
RUN apt-get update && apt-get install -y \
    net-tools \
    iputils-ping \
    tcpdump \
    nmap \
    netcat \
    telnet \
    dig \
    && rm -rf /var/lib/apt/lists/*

# Install monitoring tools
RUN pip3 install \
    httpie \
    awscli \
    ansible

# Create workspace directory
WORKDIR /workspace

# Set up shell environment
COPY bashrc /root/.bashrc
COPY vimrc /root/.vimrc

# Create useful aliases and functions
RUN echo 'alias ll="ls -la"' >> /root/.bashrc \
    && echo 'alias k="kubectl"' >> /root/.bashrc \
    && echo 'alias dc="docker-compose"' >> /root/.bashrc \
    && echo 'alias gr="go run"' >> /root/.bashrc \
    && echo 'alias gt="go test"' >> /root/.bashrc \
    && echo 'alias gl="golangci-lint run"' >> /root/.bashrc

# Add development scripts
COPY scripts/ /usr/local/bin/
RUN chmod +x /usr/local/bin/*

CMD ["/bin/bash"]
