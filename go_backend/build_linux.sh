#!/usr/bin/env bash
set -e

# ==============================================================================
# ZW-LTV Go 后台 Linux 生产环境交叉编译与打包脚本
# 目标系统: Alibaba Cloud Linux 3 (默认 x86_64 / amd64，支持 arm64)
# ==============================================================================

# 进入脚本所在目录 (go_backend)
cd "$(dirname "$0")"

TARGET_OS="linux"
TARGET_ARCH="${1:-amd64}" # 默认为 amd64，如果阿里云为 ARM(倚天710) 实例可传 arm64
OUTPUT_NAME="server"
DIST_DIR="dist"
PACKAGE_NAME="ltv-server-${TARGET_OS}-${TARGET_ARCH}.tar.gz"

echo "=========================================================="
echo "开始编译 Go 后台二进制文件..."
echo "目标系统: ${TARGET_OS}"
echo "目标架构: ${TARGET_ARCH}"
echo "编译参数: CGO_ENABLED=0, ldflags=\"-s -w\""
echo "=========================================================="

# 1. 执行静态交叉编译 (完全不依赖 CGO 和宿主机 glibc，100% 兼容 Alibaba Cloud Linux 3)
CGO_ENABLED=0 GOOS=${TARGET_OS} GOARCH=${TARGET_ARCH} go build \
    -trimpath \
    -ldflags="-s -w" \
    -o ${OUTPUT_NAME} \
    cmd/server/main.go

echo "✅ 编译成功: ./${OUTPUT_NAME}"
ls -lh ${OUTPUT_NAME}

# 2. 准备发布目录 dist
echo ""
echo "正在创建发布打包产物..."
rm -rf ${DIST_DIR}
mkdir -p ${DIST_DIR}/configs

# 复制可执行文件
cp ${OUTPUT_NAME} ${DIST_DIR}/
# 复制配置文件模板与生产配置
cp configs/config.example.yaml ${DIST_DIR}/configs/
if [ -f "configs/config.production.yaml" ]; then
    cp configs/config.production.yaml ${DIST_DIR}/configs/
fi
if [ -f "configs/config.yaml" ]; then
    cp configs/config.yaml ${DIST_DIR}/configs/config.local.yaml
fi

# 复制 Dockerfile 与部署说明
if [ -f "Dockerfile" ]; then
    cp Dockerfile ${DIST_DIR}/
fi
if [ -f "docker-compose.yaml" ]; then
    cp docker-compose.yaml ${DIST_DIR}/
fi
if [ -f "DEPLOY.md" ]; then
    cp DEPLOY.md ${DIST_DIR}/
fi

# 打包为发布归档 tar.gz
tar -czf ${PACKAGE_NAME} -C ${DIST_DIR} .
echo "✅ 打包完成: ${PACKAGE_NAME}"
ls -lh ${PACKAGE_NAME}

echo ""
echo "=========================================================="
echo "🎉 构建完成！"
echo "产物 1 (单一可执行文件): ./go_backend/${OUTPUT_NAME}"
echo "产物 2 (完整上线发布包): ./go_backend/${PACKAGE_NAME}"
echo "=========================================================="
