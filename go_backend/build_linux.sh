#!/usr/bin/env bash
set -e

# ==============================================================================
# DataLens Go 后台 Linux 生产环境交叉编译与打包脚本
# 目标系统: Alibaba Cloud Linux 3 (默认 x86_64 / amd64，支持 arm64)
# 特性说明:
#   1. 所有产物统一存放在 dist/ 目录下，根目录不留冗余文件，server 二进制仅保留一份。
#   2. 彻底禁用 macOS 扩展属性 (._*) 与 .DS_Store 打包，保证 Linux 环境绝对干净。
# ==============================================================================

# 进入脚本所在目录 (go_backend)
cd "$(dirname "$0")"

# 1. 禁用 macOS 系统的 AppleDouble 扩展属性拷贝 (避免生成 ._* 文件)
export COPYFILE_DISABLE=1

TARGET_OS="linux"
TARGET_ARCH="${1:-amd64}" # 默认为 amd64，如果阿里云为 ARM(倚天710) 实例可传 arm64
DIST_DIR="dist"
OUTPUT_NAME="server"
PACKAGE_NAME="data-lens-server-${TARGET_OS}-${TARGET_ARCH}.tar.gz"

echo "=========================================================="
echo "开始构建 Go 后台 Linux 生产发布包..."
echo "目标系统: ${TARGET_OS}"
echo "目标架构: ${TARGET_ARCH}"
echo "产物目录: $(pwd)/${DIST_DIR}"
echo "=========================================================="

# 2. 清理历史构建产物，初始化 dist 目录
rm -rf "${DIST_DIR}"
rm -f "${OUTPUT_NAME}" "${PACKAGE_NAME}"
mkdir -p "${DIST_DIR}/configs"

# 3. 执行静态交叉编译，直接输出至 dist/ 目录 (全工程仅保留这唯一一份 server 二进制)
echo "正在编译 Linux 静态二进制文件..."
CGO_ENABLED=0 GOOS=${TARGET_OS} GOARCH=${TARGET_ARCH} go build \
    -trimpath \
    -ldflags="-s -w" \
    -o "${DIST_DIR}/${OUTPUT_NAME}" \
    cmd/server/main.go

echo "✅ 编译成功: ${DIST_DIR}/${OUTPUT_NAME}"

# 4. 复制配置文件与 Docker 部署资源到 dist/ 目录
echo "正在组装发布依赖文件..."
cp configs/config.example.yaml "${DIST_DIR}/configs/"
if [ -f "configs/config.production.yaml" ]; then
    cp configs/config.production.yaml "${DIST_DIR}/configs/"
fi
if [ -f "configs/config.yaml" ]; then
    cp configs/config.yaml "${DIST_DIR}/configs/config.local.yaml"
fi
if [ -f "Dockerfile" ]; then
    cp Dockerfile "${DIST_DIR}/"
fi
if [ -f "docker-compose.yaml" ]; then
    cp docker-compose.yaml "${DIST_DIR}/"
fi

# 5. 清理待打包目录中的 macOS 隐藏文件 (.DS_Store 与 ._*)
find "${DIST_DIR}" -name ".DS_Store" -o -name "._*" -delete 2>/dev/null || true

# 6. 打包为 Linux 交付压缩包 (存放于 dist/ 目录下)
# --no-mac-metadata / --exclude 确保跨平台打包完全纯净
echo "正在生成发布压缩包..."
tar \
    --exclude=".DS_Store" \
    --exclude="._*" \
    --exclude="*.tar.gz" \
    -czf "${DIST_DIR}/${PACKAGE_NAME}" \
    -C "${DIST_DIR}" .

echo "✅ 打包完成: ${DIST_DIR}/${PACKAGE_NAME}"

echo ""
echo "=========================================================="
echo "🎉 构建完成！所有产物已统一收拢至 [go_backend/dist/]:"
echo "  - 二进制文件 (唯一):  ./dist/${OUTPUT_NAME}"
echo "  - 线上发布包:         ./dist/${PACKAGE_NAME}"
echo "  - 配置文件与部署配置: ./dist/configs/ , ./dist/docker-compose.yaml 等"
echo "=========================================================="
ls -lh "${DIST_DIR}/${OUTPUT_NAME}" "${DIST_DIR}/${PACKAGE_NAME}"
