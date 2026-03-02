# Deployment Guide

This guide covers deploying abot in various environments, from development to production.

## Prerequisites

- **Docker** (recommended) or **Go 1.21+**
- **MySQL 8.0+** (for server/web modes)
- **LLM API Key** (OpenAI, Anthropic, or compatible provider)

## Quick Start

### Development (CLI Mode)

Fastest way to get started, no database required:

```bash
# Download binary
wget https://github.com/yourusername/abot/releases/latest/download/abot-agent-linux-amd64.tar.gz
tar -xzf abot-agent-linux-amd64.tar.gz
sudo mv abot-agent /usr/local/bin/

# Create minimal config
cat > config.yaml <<EOF
providers:
  - api_key: sk-xxx
    model: gpt-4o-mini
EOF

# Run
abot-agent -config config.yaml
```

### Production (Web Mode)

Full-featured deployment with database:

```bash
# Using Docker Compose (recommended)
curl -O https://raw.githubusercontent.com/yourusername/abot/main/docker-compose.yml

# Configure environment
cat > .env <<EOF
OPENAI_API_KEY=sk-xxx
JWT_SECRET=$(openssl rand -hex 32)
MYSQL_ROOT_PASSWORD=secure-password
EOF

# Start services
docker-compose up -d

# Check status
docker-compose ps
docker-compose logs -f abot-web
```

## Deployment Options

### 1. Docker (Recommended)

#### Single Container (Web Mode)

```bash
docker run -d \
  --name abot-web \
  -p 3000:3000 \
  -e MYSQL_DSN="user:pass@tcp(mysql:3306)/abot?charset=utf8mb4&parseTime=True" \
  -e OPENAI_API_KEY="sk-xxx" \
  -e JWT_SECRET="$(openssl rand -hex 32)" \
  abot/web:latest
```

#### Docker Compose (Full Stack)

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD}
      MYSQL_DATABASE: abot
      MYSQL_USER: abot
      MYSQL_PASSWORD: ${MYSQL_PASSWORD}
    volumes:
      - mysql_data:/var/lib/mysql
    ports:
      - "3306:3306"
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      interval: 10s
      timeout: 5s
      retries: 5

  abot-web:
    image: abot/web:latest
    depends_on:
      mysql:
        condition: service_healthy
    environment:
      MYSQL_DSN: "abot:${MYSQL_PASSWORD}@tcp(mysql:3306)/abot?charset=utf8mb4&parseTime=True"
      OPENAI_API_KEY: ${OPENAI_API_KEY}
      JWT_SECRET: ${JWT_SECRET}
    ports:
      - "3000:3000"
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./data:/app/data
    restart: unless-stopped

volumes:
  mysql_data:
```

Create `.env`:

```bash
MYSQL_ROOT_PASSWORD=secure-root-password
MYSQL_PASSWORD=secure-abot-password
OPENAI_API_KEY=sk-xxx
JWT_SECRET=$(openssl rand -hex 32)
```

Start:

```bash
docker-compose up -d
```

### 2. Kubernetes

#### Using Helm Chart

```bash
# Add Helm repository
helm repo add abot https://charts.abot.run
helm repo update

# Install with default values
helm install abot abot/abot \
  --set openai.apiKey=sk-xxx \
  --set mysql.enabled=true \
  --set ingress.enabled=true \
  --set ingress.host=abot.example.com

# Or use custom values file
cat > values.yaml <<EOF
openai:
  apiKey: sk-xxx

mysql:
  enabled: true
  auth:
    rootPassword: secure-password
    database: abot
    username: abot
    password: secure-password

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: abot.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: abot-tls
      hosts:
        - abot.example.com

replicaCount: 3

resources:
  limits:
    cpu: 2000m
    memory: 2Gi
  requests:
    cpu: 500m
    memory: 512Mi
EOF

helm install abot abot/abot -f values.yaml
```

#### Manual Kubernetes Deployment

Create `k8s/namespace.yaml`:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: abot
```

Create `k8s/secrets.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: abot-secrets
  namespace: abot
type: Opaque
stringData:
  mysql-dsn: "abot:password@tcp(mysql:3306)/abot?charset=utf8mb4&parseTime=True"
  openai-key: "sk-xxx"
  jwt-secret: "your-jwt-secret-here"
```

Create `k8s/deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: abot-web
  namespace: abot
spec:
  replicas: 3
  selector:
    matchLabels:
      app: abot-web
  template:
    metadata:
      labels:
        app: abot-web
    spec:
      containers:
      - name: abot-web
        image: abot/web:latest
        ports:
        - containerPort: 3000
          name: http
        env:
        - name: MYSQL_DSN
          valueFrom:
            secretKeyRef:
              name: abot-secrets
              key: mysql-dsn
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: abot-secrets
              key: openai-key
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: abot-secrets
              key: jwt-secret
        livenessProbe:
          httpGet:
            path: /health
            port: 3000
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 3000
          initialDelaySeconds: 10
          periodSeconds: 5
        resources:
          limits:
            cpu: 2000m
            memory: 2Gi
          requests:
            cpu: 500m
            memory: 512Mi
```

Create `k8s/service.yaml`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: abot-web
  namespace: abot
spec:
  selector:
    app: abot-web
  ports:
  - port: 80
    targetPort: 3000
    protocol: TCP
  type: LoadBalancer
```

Create `k8s/ingress.yaml`:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: abot-web
  namespace: abot
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/proxy-body-size: "50m"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - abot.example.com
    secretName: abot-tls
  rules:
  - host: abot.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: abot-web
            port:
              number: 80
```

Deploy:

```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/secrets.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/ingress.yaml

# Check status
kubectl get pods -n abot
kubectl logs -f -n abot deployment/abot-web
```

### 3. Binary (Bare Metal / VM)

#### Install Binary

```bash
# Download latest release
wget https://github.com/yourusername/abot/releases/latest/download/abot-web-linux-amd64.tar.gz
tar -xzf abot-web-linux-amd64.tar.gz
sudo mv abot-web /usr/local/bin/

# Verify installation
abot-web -version
```

#### Create Configuration

```bash
# Create config directory
sudo mkdir -p /etc/abot

# Create config file
sudo cat > /etc/abot/config.yaml <<EOF
app_name: abot
mysql_dsn: "abot:password@tcp(localhost:3306)/abot?charset=utf8mb4&parseTime=True"

providers:
  - api_key: sk-xxx
    model: gpt-4o-mini

console:
  addr: ":3000"
  jwt_secret: "$(openssl rand -hex 32)"
  static_dir: "/var/www/abot"
  allowed_origins:
    - "https://abot.example.com"

agents:
  - id: default-bot
    name: "AI Assistant"
    model: gpt-4o-mini
EOF

# Set permissions
sudo chmod 600 /etc/abot/config.yaml
```

#### Create Systemd Service

```bash
# Create service file
sudo cat > /etc/systemd/system/abot-web.service <<EOF
[Unit]
Description=abot Web Console
After=network.target mysql.service
Wants=mysql.service

[Service]
Type=simple
User=abot
Group=abot
WorkingDirectory=/var/lib/abot
ExecStart=/usr/local/bin/abot-web -config /etc/abot/config.yaml
Restart=on-failure
RestartSec=5s

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/abot /var/log/abot

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
EOF

# Create user
sudo useradd -r -s /bin/false abot

# Create directories
sudo mkdir -p /var/lib/abot /var/log/abot
sudo chown -R abot:abot /var/lib/abot /var/log/abot

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable abot-web
sudo systemctl start abot-web

# Check status
sudo systemctl status abot-web
sudo journalctl -u abot-web -f
```

## Database Setup

### MySQL

#### Create Database

```bash
mysql -u root -p <<EOF
CREATE DATABASE abot CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'abot'@'%' IDENTIFIED BY 'secure-password';
GRANT ALL PRIVILEGES ON abot.* TO 'abot'@'%';
FLUSH PRIVILEGES;
EOF
```

#### Test Connection

```bash
mysql -u abot -p abot -e "SELECT 1"
```

#### Migrations

Migrations run automatically on startup. To run manually:

```bash
abot-web -config config.yaml -migrate-only
```

#### Backup and Restore

```bash
# Backup
mysqldump -u abot -p abot > backup-$(date +%Y%m%d).sql

# Restore
mysql -u abot -p abot < backup-20260302.sql
```

## Reverse Proxy

### Nginx

Create `/etc/nginx/sites-available/abot`:

```nginx
upstream abot_backend {
    server localhost:3000;
    keepalive 32;
}

server {
    listen 80;
    server_name abot.example.com;

    # Redirect to HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name abot.example.com;

    # SSL certificates
    ssl_certificate /etc/letsencrypt/live/abot.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/abot.example.com/privkey.pem;

    # SSL configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;

    # Proxy settings
    location / {
        proxy_pass http://abot_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # WebSocket support
    location /api/ws {
        proxy_pass http://abot_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "Upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket timeouts
        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;
    }

    # Static files (if serving separately)
    location /static/ {
        alias /var/www/abot/static/;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # Logging
    access_log /var/log/nginx/abot-access.log;
    error_log /var/log/nginx/abot-error.log;
}
```

Enable and reload:

```bash
sudo ln -s /etc/nginx/sites-available/abot /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### Caddy

Create `Caddyfile`:

```
abot.example.com {
    reverse_proxy localhost:3000

    # WebSocket support (automatic)

    # Security headers
    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        X-Frame-Options "SAMEORIGIN"
        X-Content-Type-Options "nosniff"
        X-XSS-Protection "1; mode=block"
    }

    # Logging
    log {
        output file /var/log/caddy/abot.log
    }
}
```

Start Caddy:

```bash
sudo caddy run --config Caddyfile
```

## SSL/TLS

### Let's Encrypt (Certbot)

```bash
# Install certbot
sudo apt install certbot python3-certbot-nginx

# Get certificate (Nginx)
sudo certbot --nginx -d abot.example.com

# Get certificate (standalone)
sudo certbot certonly --standalone -d abot.example.com

# Auto-renewal (test)
sudo certbot renew --dry-run

# Auto-renewal (cron)
sudo crontab -e
# Add: 0 3 * * * certbot renew --quiet
```

### Custom Certificate

```bash
# Generate self-signed certificate (development only)
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/ssl/private/abot.key \
  -out /etc/ssl/certs/abot.crt \
  -subj "/CN=abot.example.com"
```

## Monitoring

### Prometheus

Create `prometheus.yml`:

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'abot'
    static_configs:
      - targets: ['localhost:3000']
    metrics_path: '/metrics'
```

Start Prometheus:

```bash
docker run -d \
  --name prometheus \
  -p 9090:9090 \
  -v $(pwd)/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus
```

### Grafana

```bash
# Start Grafana
docker run -d \
  --name grafana \
  -p 3001:3000 \
  grafana/grafana

# Access: http://localhost:3001
# Default credentials: admin/admin

# Add Prometheus data source
# URL: http://prometheus:9090

# Import dashboard (ID to be created)
```

### Health Checks

```bash
# Basic health check
curl http://localhost:3000/health

# Detailed health check
curl http://localhost:3000/health/detailed

# Metrics
curl http://localhost:3000/metrics
```

## Backup Strategy

### Database Backup

```bash
# Daily backup script
cat > /usr/local/bin/backup-abot.sh <<'EOF'
#!/bin/bash
BACKUP_DIR="/var/backups/abot"
DATE=$(date +%Y%m%d_%H%M%S)
mkdir -p $BACKUP_DIR

# Backup database
mysqldump -u abot -p$MYSQL_PASSWORD abot | gzip > $BACKUP_DIR/abot-db-$DATE.sql.gz

# Keep only last 30 days
find $BACKUP_DIR -name "abot-db-*.sql.gz" -mtime +30 -delete

echo "Backup completed: $BACKUP_DIR/abot-db-$DATE.sql.gz"
EOF

chmod +x /usr/local/bin/backup-abot.sh

# Add to crontab
crontab -e
# Add: 0 2 * * * /usr/local/bin/backup-abot.sh
```

### Configuration Backup

```bash
# Backup config
tar -czf abot-config-$(date +%Y%m%d).tar.gz /etc/abot/

# Restore
tar -xzf abot-config-20260302.tar.gz -C /
```

### Automated Backup to S3

```bash
# Install AWS CLI
sudo apt install awscli

# Configure AWS credentials
aws configure

# Backup script with S3 upload
cat > /usr/local/bin/backup-abot-s3.sh <<'EOF'
#!/bin/bash
BACKUP_DIR="/var/backups/abot"
DATE=$(date +%Y%m%d_%H%M%S)
S3_BUCKET="s3://your-bucket/abot-backups"

# Backup database
mysqldump -u abot -p$MYSQL_PASSWORD abot | gzip > $BACKUP_DIR/abot-db-$DATE.sql.gz

# Upload to S3
aws s3 cp $BACKUP_DIR/abot-db-$DATE.sql.gz $S3_BUCKET/

# Clean local backups (keep 7 days)
find $BACKUP_DIR -name "abot-db-*.sql.gz" -mtime +7 -delete

echo "Backup uploaded to S3: $S3_BUCKET/abot-db-$DATE.sql.gz"
EOF

chmod +x /usr/local/bin/backup-abot-s3.sh
```

## Scaling

### Horizontal Scaling (Kubernetes)

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: abot-web
  namespace: abot
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: abot-web
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### Database Scaling

#### Read Replicas

```yaml
# MySQL with read replicas
mysql-primary:
  image: mysql:8.0
  environment:
    MYSQL_ROOT_PASSWORD: password
  volumes:
    - mysql-primary-data:/var/lib/mysql

mysql-replica-1:
  image: mysql:8.0
  environment:
    MYSQL_ROOT_PASSWORD: password
    MYSQL_REPLICATION_MODE: slave
    MYSQL_MASTER_HOST: mysql-primary
  volumes:
    - mysql-replica-1-data:/var/lib/mysql
```

#### Connection Pooling

Configure in `config.yaml`:

```yaml
mysql_dsn: "abot:password@tcp(localhost:3306)/abot?charset=utf8mb4&parseTime=True"
mysql_max_open_conns: 100
mysql_max_idle_conns: 10
mysql_conn_max_lifetime: 3600
```

## Troubleshooting

### Service Won't Start

```bash
# Check logs
sudo journalctl -u abot-web -n 100 --no-pager

# Check config
abot-web -config /etc/abot/config.yaml -validate

# Check database connection
mysql -u abot -p -h localhost abot -e "SELECT 1"

# Check port availability
sudo netstat -tlnp | grep 3000
```

### High Memory Usage

```bash
# Check memory usage
docker stats abot-web

# Increase memory limit (Docker)
docker run -m 2g abot/web:latest

# Increase memory limit (Kubernetes)
# Edit deployment.yaml resources.limits.memory
```

### Slow Response Times

```bash
# Check database queries
mysql -u abot -p -e "SHOW PROCESSLIST"

# Enable slow query log
mysql -u root -p -e "SET GLOBAL slow_query_log = 'ON'; SET GLOBAL long_query_time = 1;"

# Check slow queries
tail -f /var/log/mysql/slow-query.log

# Add database indexes
mysql -u abot -p abot -e "SHOW INDEX FROM sessions"
```

### WebSocket Connection Issues

```bash
# Check Nginx WebSocket config
sudo nginx -t

# Test WebSocket connection
wscat -c ws://localhost:3000/api/ws?token=YOUR_JWT_TOKEN

# Check firewall
sudo ufw status
sudo ufw allow 3000/tcp
```

## Security Checklist

- [ ] Change default JWT secret
- [ ] Use strong database password
- [ ] Enable HTTPS (SSL/TLS)
- [ ] Configure firewall (allow only necessary ports)
- [ ] Regular security updates
- [ ] Backup encryption
- [ ] API rate limiting
- [ ] Input validation
- [ ] Disable debug mode in production
- [ ] Use environment variables for secrets
- [ ] Enable audit logging
- [ ] Implement IP whitelisting (if applicable)

## Production Checklist

- [ ] Database backups configured and tested
- [ ] Monitoring enabled (Prometheus/Grafana)
- [ ] Logging configured (centralized logging)
- [ ] SSL/TLS enabled and auto-renewal configured
- [ ] Health checks configured
- [ ] Auto-scaling configured (if using Kubernetes)
- [ ] Disaster recovery plan documented
- [ ] Documentation updated
- [ ] Load testing completed
- [ ] Security audit completed
- [ ] Rollback plan prepared
- [ ] Team trained on operations

---

**Last Updated**: 2026-03-02
**Version**: 2.0
