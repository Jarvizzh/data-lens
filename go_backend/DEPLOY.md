# ZW-LTV Go 后台 Docker 容器化部署指引 (Alibaba Cloud Linux 3)

本文档指导如何将已在本地构建好的 Go 后端服务通过 **Docker** 容器化发布并部署至 **Alibaba Cloud Linux 3** 生产服务器。

---

## 一、 构建产物说明

本项目采用纯静态编译（`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`），**无需在生产服务器上安装 Go 环境或拉取依赖构建**。

在本地开发机执行构建：
```bash
cd go_backend
./build_linux.sh
```
执行完成后将生成：
1. **`server`**：Linux 64位无依赖静态二进制文件（约 24MB）。
2. **`ltv-server-linux-amd64.tar.gz`**：上线发布压缩包（约 8.9MB），包含可执行文件、Dockerfile、docker-compose.yaml 以及生产配置模板。

---

## 二、 生产配置核对清单 (Pre-flight Checklist)

在启动容器前，请核查确认 `configs/config.production.yaml` 配置：

1. **服务端口配置（生产环境默认为 8098，开发环境为 8080）**：
   ```yaml
   server:
     port: 8098
   ```
2. **MySQL 数据库配置**：
   ```yaml
   database:
     mysql:
       dsn: "user:password@tcp(rm-xxx.mysql.rds.aliyuncs.com:3306)/meta_ltv?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai"
   ```
   - 确认云数据库 RDS 的白名单已放行该 ECS 服务器内网 IP。
3. **第三方 API 配置**：
   - `order.api`（中文在线）：确认线上有效 Authorization / Token。
   - `flicknovel.api`（番茄司南）：确认线上有效 Private Key 与 Company ID。
4. **日志输出模式**：
   ```yaml
   logger:
     level: "info"
     format: "json"      # 生产环境推荐 json 格式便于阿里云 SLS 日志服务采集
     show_caller: true
   ```

---

## 三、 Docker 部署全流程

### 步骤 1：上传发布包至生产服务器
在本地终端执行：
```bash
scp go_backend/ltv-server-linux-amd64.tar.gz root@<ECS_IP>:/data/
```

### 步骤 2：解压发布包
登录服务器并进入部署目录：
```bash
ssh root@<ECS_IP>
mkdir -p /data/ltv-server
tar -zxvf /data/ltv-server-linux-amd64.tar.gz -C /data/ltv-server
cd /data/ltv-server
```

### 步骤 3：构建轻量运行镜像
> **说明**：二进制文件已在本地编译完成，此步骤仅基于 Alpine 打包时区和 CA 根证书，构建仅需几秒钟，生成的镜像体积仅约 30MB。

```bash
docker build -t ltv-server:latest .
```

### 步骤 4：启动容器

#### 方式 A：使用 Docker Compose（推荐）
检查并确保 `/data/ltv-server/configs/config.production.yaml` 的数据库与密钥无误，执行：
```bash
docker compose up -d
```

#### 方式 B：使用 `docker run` 原生命令
```bash
docker run -d \
  --name ltv-server \
  --restart always \
  -p 8098:8098 \
  -v /data/ltv-server/configs/config.production.yaml:/app/configs/config.yaml:ro \
  -e TZ=Asia/Shanghai \
  -e SERVER_PORT=8098 \
  ltv-server:latest
```

### 步骤 5：验证服务状态与日志
```bash
# 查看容器运行状态
docker ps | grep ltv-server

# 查看实时日志
docker logs -f --tail 100 ltv-server

# 容器本地健康检查测试 (端口: 8098)
curl -i http://127.0.0.1:8098/api/auth/profile
```

---

## 四、 Nginx 反向代理配置参考

通常线上由 Nginx 统一处理域名解析、HTTPS 证书，并将 API 请求转发到容器监听的 `8098` 端口：

```nginx
server {
    listen 80;
    server_name ltv.yourdomain.com;
    # 若配置 SSL 可开启强制跳转
    # return 301 https://$host$request_uri;

    # 前端静态资源
    location / {
        root /data/frontend/dist;
        index index.html;
        try_files $uri $uri/ /index.html;
    }

    # 后端 Go API 代理 (转发给 Docker 容器 8098 端口)
    location /api/ {
        proxy_pass http://127.0.0.1:8098;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # 长连接优化
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_read_timeout 120s;
        proxy_send_timeout 120s;
    }
}
```

配置生效：
```bash
nginx -t && systemctl reload nginx
```

---

## 五、 Docker 容器更新与回滚

### 1. 版本平滑更新
后续业务迭代更新时：
```bash
# 1. 本地编译打包并上传新的 tar.gz
scp go_backend/ltv-server-linux-amd64.tar.gz root@<ECS_IP>:/data/

# 2. 服务器解压并重新构建镜像
cd /data/ltv-server
tar -zxvf /data/ltv-server-linux-amd64.tar.gz -C /data/ltv-server
docker build -t ltv-server:latest .

# 3. 重启容器
docker compose down && docker compose up -d
# 或使用 docker restart / run 命令
```

### 2. 异常快速回滚
如果更新后发现异常需要回滚，建议在上线前打 tag 备份镜像：
```bash
# 上线前备份当前运行正常的镜像
docker tag ltv-server:latest ltv-server:bak

# 如需回滚，直接重新运行备份镜像
docker stop ltv-server && docker rm ltv-server
docker run -d \
  --name ltv-server \
  --restart always \
  -p 8098:8098 \
  -v /data/ltv-server/configs/config.production.yaml:/app/configs/config.yaml:ro \
  -e TZ=Asia/Shanghai \
  -e SERVER_PORT=8098 \
  ltv-server:bak
```
