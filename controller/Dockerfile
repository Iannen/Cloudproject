FROM python:3.12-slim

# Install curl to fetch binaries
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*

# 1. Download stable tailscale binaries, extract, and copy into system PATH
RUN TS_VER="1.68.0" && \
    curl -sLO https://pkgs.tailscale.com/stable/tailscale_${TS_VER}_amd64.tgz && \
    tar -xf tailscale_${TS_VER}_amd64.tgz && \
    mv tailscale_${TS_VER}_amd64/tailscale /usr/local/bin/tailscale && \
    rm -rf tailscale_${TS_VER}_amd64*

# 2. Install Docker CLI and Docker Compose Plugin (v2 format)
RUN DOCKER_VER="26.1.4" && \
    curl -sLO https://download.docker.com/linux/static/stable/x86_64/docker-${DOCKER_VER}.tgz && \
    tar -xf docker-${DOCKER_VER}.tgz && \
    mv docker/docker /usr/local/bin/docker && \
    rm -rf docker* && \
    # Set up the docker compose CLI plugin directory structure
    mkdir -p /usr/local/lib/docker/cli-plugins && \
    COMPOSE_VER="v2.27.1" && \
    curl -SL https://github.com/docker/compose/releases/download/${COMPOSE_VER}/docker-compose-linux-x86_64 -o /usr/local/lib/docker/cli-plugins/docker-compose && \
    chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

WORKDIR /app

COPY build-dir/ /app/

RUN if [ -f requirements.txt ]; then \
        pip install --no-cache-dir -r requirements.txt; \
    fi

ENV PYTHONUNBUFFERED=1

CMD ["python", "main.py"]
