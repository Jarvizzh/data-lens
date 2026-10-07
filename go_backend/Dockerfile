# ==============================================================================
# ZW-LTV Go 后端生产环境轻量 Dockerfile
# 说明: 直接基于本地预编译好的 Linux 静态可执行文件 (CGO_ENABLED=0)，无需在容器中安装 Go 环境
# ==============================================================================

FROM alpine:3.20

# 维护者信息
LABEL maintainer="Jarvis"

# 1. 替换阿里云镜像源并安装 HTTPS 根证书与时区数据
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

# 2. 设置时区与环境变量 (生产环境默认端口: 8098)
ENV TZ=Asia/Shanghai \
    SERVER_PORT=8098 \
    CONFIG_PATH=/app/configs/config.yaml

WORKDIR /app

# 3. 复制静态二进制文件与配置模板
COPY server /app/server
COPY configs /app/configs

# 4. 确保文件具有可执行权限
RUN chmod +x /app/server

# 5. 声明服务端口 (生产环境: 8098)
EXPOSE 8098

# 6. 容器启动命令
ENTRYPOINT ["/app/server"]
