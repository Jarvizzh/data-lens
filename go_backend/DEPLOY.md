# DataLens (data-lens-server) Go 后台 Docker 容器化部署指引 (Alibaba Cloud Linux 3)

本文档指导如何将已在本地构建好的 Go 后端服务通过 **Docker** 容器化发布并部署至 **Alibaba Cloud Linux 3** 生产服务器。

---

## 一、 构建产物说明

本项目采用纯静态编译（`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`），**无需在生产服务器上安装 Go 环境或拉取依赖构建**。

在本地开发机执行构建：
```bash
cd go_backend
./build_linux.sh
```
执行完成后，所有产物将统一存放于 **`go_backend/dist/`** 目录：
1. **`dist/server`**：Linux 64位无依赖静态二进制文件（约 24MB，全工程唯一保留一份）。
2. **`dist/data-lens-server-linux-amd64.tar.gz`**：上线发布压缩包（约 8.9MB），已彻底过滤 macOS 隐藏文件。

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
scp go_backend/dist/data-lens-server-linux-amd64.tar.gz root@<ECS_IP>:/data/
```

### 步骤 2：解压发布包
登录服务器并进入部署目录：
```bash
ssh root@<ECS_IP>
mkdir -p /data/data-lens-server
tar -zxvf /data/data-lens-server-linux-amd64.tar.gz -C /data/data-lens-server
cd /data/data-lens-server
```

### 步骤 3：构建轻量运行镜像
> **说明**：二进制文件已在本地编译完成，此步骤仅基于 Alpine 打包时区和 CA 根证书，构建仅需几秒钟，生成的镜像体积仅约 30MB。

```bash
docker build -t data-lens-server:latest .
```

### 步骤 4：启动容器

#### 方式 A：使用 Docker Compose（推荐）
检查并确保 `/data/data-lens-server/configs/config.production.yaml` 的数据库与密钥无误，执行：
```bash
docker compose up -d
```

#### 方式 B：使用 `docker run` 原生命令 (针对宿主机本地 MySQL 推荐使用 --network host)
```bash
docker run -d \
  --name data-lens-server \
  --restart always \
  --network host \
  -v /data/data-lens-server/configs/config.production.yaml:/app/configs/config.yaml:ro \
  -e TZ=Asia/Shanghai \
  -e SERVER_PORT=8098 \
  data-lens-server:latest
```

### 步骤 5：验证服务状态与日志
```bash
# 查看容器运行状态
docker ps | grep data-lens-server

# 查看实时日志
docker logs -f --tail 100 data-lens-server

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
        
        # 长连接与大跨度数据同步超时优化 (避免全量历史同步触发 Nginx 默认 60s 截断)
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
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
scp go_backend/data-lens-server-linux-amd64.tar.gz root@<ECS_IP>:/data/

# 2. 服务器解压并重新构建镜像
cd /data/data-lens-server
tar -zxvf /data/data-lens-server-linux-amd64.tar.gz -C /data/data-lens-server
docker build -t data-lens-server:latest .

# 3. 重启容器
docker compose down && docker compose up -d
# 或使用 docker restart / run 命令
```

### 2. 异常快速回滚
如果更新后发现异常需要回滚，建议在上线前打 tag 备份镜像：
```bash
# 上线前备份当前运行正常的镜像
docker tag data-lens-server:latest data-lens-server:bak

# 如需回滚，直接重新运行备份镜像
docker stop data-lens-server && docker rm data-lens-server
docker run -d \
  --name data-lens-server \
  --restart always \
  -p 8098:8098 \
  -v /data/data-lens-server/configs/config.production.yaml:/app/configs/config.yaml:ro \
  -e TZ=Asia/Shanghai \
  -e SERVER_PORT=8098 \
  data-lens-server:bak
```

---

## 六、 常见问题与排查 (FAQ)

### Q: 启动报 `dial tcp [::1]:3306: connect: connection refused`
**原因分析**：
在 Docker 容器内部，`localhost` 或 `127.0.0.1` 指向容器自身（而不是 ECS 宿主机）。因为容器内部没有运行 MySQL，连本地 3306 必然会报拒绝连接。

**解决方案（根据您的 MySQL 安装位置选择）**：

#### 情况 1：MySQL 与本服务在同一台 ECS 宿主机上
- **方案 A (最推荐，性能最高)**：在 Linux 上使用 `host` 网络模式，容器与宿主机共享网络，`localhost:3306` 即可直连宿主机 MySQL：
  ```bash
  docker run -d \
    --name data-lens-server \
    --restart always \
    --network host \
    -v /data/data-lens-server/configs/config.production.yaml:/app/configs/config.yaml:ro \
    -e TZ=Asia/Shanghai \
    -e SERVER_PORT=8098 \
    data-lens-server:latest
  ```
  *(注：使用 `--network host` 时无需 `-p 8098:8098` 端口映射，端口直接在宿主机开放)*
- **方案 B**：使用宿主机网关 `host.docker.internal`：
  修改 `configs/config.production.yaml` 中的 DSN：
  ```yaml
  dsn: "root:你的密码@tcp(host.docker.internal:3306)/meta_ltv?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai"
  ```
  或者使用 ECS 的私网 IP（如 `172.x.x.x:3306`）。

#### 情况 2：MySQL 是阿里云 RDS (云数据库)
- 请检查并修改 `/data/data-lens-server/configs/config.production.yaml`，将 `localhost:3306` 替换为 RDS 真实内网连接地址：
  ```yaml
  dsn: "用户名:密码@tcp(rm-xxxxxx.mysql.rds.aliyuncs.com:3306)/meta_ltv?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai"
  ```
- 确认 RDS 白名单已加入该 ECS 服务器的私网 IP。

#### 快捷方案：通过环境变量直接注入（无需改配置文件）
在 `docker run` 时传入 `-e MYSQL_DSN="..."` 即可即时覆盖：
```bash
docker run -d \
  --name data-lens-server \
  --restart always \
  -p 8098:8098 \
  -e MYSQL_DSN="root:你的密码@tcp(rm-xxx.mysql.rds.aliyuncs.com:3306)/meta_ltv?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai" \
  -e TZ=Asia/Shanghai \
  -e SERVER_PORT=8098 \
  data-lens-server:latest
```

