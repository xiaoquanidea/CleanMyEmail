#!/bin/bash

# CleanMyEmail 多平台构建脚本
# 用法: ./scripts/build.sh [platform] [version]
# 平台: darwin-amd64, darwin-arm64, windows-amd64, linux-amd64, all
# 示例: ./scripts/build.sh all 1.0.0

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 项目信息
APP_NAME="CleanMyEmail"
BUILD_DIR="build/bin"

# 版本信息
VERSION="${2:-1.0.0}"
BUILD_TIME=$(date +"%Y-%m-%d %H:%M:%S")

# ldflags
LDFLAGS="-X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}'"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  CleanMyEmail 多平台构建脚本${NC}"
echo -e "${GREEN}  版本: ${VERSION}${NC}"
echo -e "${GREEN}  构建时间: ${BUILD_TIME}${NC}"
echo -e "${GREEN}========================================${NC}"

# 检查 wails 是否安装
check_wails() {
    if ! command -v wails &> /dev/null; then
        echo -e "${RED}错误: wails 未安装${NC}"
        echo "请运行: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
        exit 1
    fi
    echo -e "${GREEN}✓ wails 已安装${NC}"
}

# 构建函数
build_platform() {
    local platform=$1
    local arch=$2
    local output_name=$3
    
    echo -e "\n${YELLOW}正在构建 ${platform}/${arch}...${NC}"
    
    # 设置输出目录
    local output_dir="${BUILD_DIR}/${platform}-${arch}"
    mkdir -p "${output_dir}"
    
    # 构建命令
    if [ "$platform" == "darwin" ]; then
        GOOS=darwin GOARCH=${arch} wails build -platform darwin/${arch} -ldflags "${LDFLAGS}" -o "${output_name}"
        # 移动 .app 到输出目录
        if [ -d "${BUILD_DIR}/${APP_NAME}.app" ]; then
            mv "${BUILD_DIR}/${APP_NAME}.app" "${output_dir}/"
        fi
    elif [ "$platform" == "windows" ]; then
        GOOS=windows GOARCH=${arch} wails build -platform windows/${arch} -ldflags "${LDFLAGS}" -o "${output_name}.exe" -nsis
        # 移动文件到输出目录
        if [ -f "${BUILD_DIR}/${output_name}.exe" ]; then
            mv "${BUILD_DIR}/${output_name}.exe" "${output_dir}/"
        fi
        # 移动安装包
        if [ -f "${BUILD_DIR}/${APP_NAME}-${arch}-installer.exe" ]; then
            mv "${BUILD_DIR}/${APP_NAME}-${arch}-installer.exe" "${output_dir}/"
        fi
    elif [ "$platform" == "linux" ]; then
        GOOS=linux GOARCH=${arch} wails build -platform linux/${arch} -ldflags "${LDFLAGS}" -o "${output_name}"
        if [ -f "${BUILD_DIR}/${output_name}" ]; then
            mv "${BUILD_DIR}/${output_name}" "${output_dir}/"
        fi
    fi
    
    echo -e "${GREEN}✓ ${platform}/${arch} 构建完成${NC}"
}

# 打包函数
package_release() {
    local platform=$1
    local arch=$2
    
    local output_dir="${BUILD_DIR}/${platform}-${arch}"
    local archive_name="${APP_NAME}-${VERSION}-${platform}-${arch}"
    
    echo -e "${YELLOW}正在打包 ${archive_name}...${NC}"
    
    cd "${BUILD_DIR}"
    
    if [ "$platform" == "darwin" ]; then
        # macOS 使用 zip
        zip -r "${archive_name}.zip" "${platform}-${arch}/${APP_NAME}.app"
    elif [ "$platform" == "windows" ]; then
        # Windows 使用 zip
        zip -r "${archive_name}.zip" "${platform}-${arch}/"
    else
        # Linux 使用 tar.gz
        tar -czvf "${archive_name}.tar.gz" "${platform}-${arch}/"
    fi
    
    cd - > /dev/null
    echo -e "${GREEN}✓ ${archive_name} 打包完成${NC}"
}

# 主逻辑
main() {
    local target="${1:-all}"
    
    check_wails
    
    # 清理旧构建
    echo -e "\n${YELLOW}清理旧构建...${NC}"
    rm -rf "${BUILD_DIR}"
    mkdir -p "${BUILD_DIR}"
    
    case "$target" in
        "darwin-amd64")
            build_platform "darwin" "amd64" "${APP_NAME}"
            package_release "darwin" "amd64"
            ;;
        "darwin-arm64")
            build_platform "darwin" "arm64" "${APP_NAME}"
            package_release "darwin" "arm64"
            ;;
        "windows-amd64")
            build_platform "windows" "amd64" "${APP_NAME}"
            package_release "windows" "amd64"
            ;;
        "linux-amd64")
            build_platform "linux" "amd64" "${APP_NAME}"
            package_release "linux" "amd64"
            ;;
        "all")
            build_platform "darwin" "amd64" "${APP_NAME}"
            package_release "darwin" "amd64"
            build_platform "darwin" "arm64" "${APP_NAME}"
            package_release "darwin" "arm64"
            build_platform "windows" "amd64" "${APP_NAME}"
            package_release "windows" "amd64"
            build_platform "linux" "amd64" "${APP_NAME}"
            package_release "linux" "amd64"
            ;;
        *)
            echo -e "${RED}未知平台: ${target}${NC}"
            echo "支持的平台: darwin-amd64, darwin-arm64, windows-amd64, linux-amd64, all"
            exit 1
            ;;
    esac
    
    echo -e "\n${GREEN}========================================${NC}"
    echo -e "${GREEN}  构建完成！${NC}"
    echo -e "${GREEN}  输出目录: ${BUILD_DIR}${NC}"
    echo -e "${GREEN}========================================${NC}"
    ls -la "${BUILD_DIR}"
}

main "$@"

