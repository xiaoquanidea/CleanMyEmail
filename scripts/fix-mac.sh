#!/bin/bash

# CleanMyEmail Mac 修复脚本
# 用于解决 macOS 上无法打开未签名应用的问题
# 适用于 Intel (x86_64) 和 Apple Silicon (M1/M2/M3) Mac

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

APP_NAME="CleanMyEmail"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  CleanMyEmail Mac 修复工具${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# 检测系统架构
ARCH=$(uname -m)
if [ "$ARCH" == "arm64" ]; then
    echo -e "${GREEN}✓ 检测到 Apple Silicon (M1/M2/M3) Mac${NC}"
else
    echo -e "${GREEN}✓ 检测到 Intel Mac${NC}"
fi
echo ""

# 查找应用路径
find_app() {
    # 常见安装位置
    local locations=(
        "/Applications/${APP_NAME}.app"
        "$HOME/Applications/${APP_NAME}.app"
        "$HOME/Desktop/${APP_NAME}.app"
        "$HOME/Downloads/${APP_NAME}.app"
        "./${APP_NAME}.app"
    )
    
    for loc in "${locations[@]}"; do
        if [ -d "$loc" ]; then
            echo "$loc"
            return 0
        fi
    done
    
    return 1
}

# 主修复逻辑
fix_app() {
    local app_path="$1"
    
    echo -e "${YELLOW}正在修复: ${app_path}${NC}"
    echo ""
    
    # 1. 移除隔离属性 (quarantine)
    echo -e "${BLUE}[1/3] 移除隔离属性...${NC}"
    if xattr -d com.apple.quarantine "$app_path" 2>/dev/null; then
        echo -e "${GREEN}  ✓ 隔离属性已移除${NC}"
    else
        echo -e "${YELLOW}  - 无隔离属性或已移除${NC}"
    fi
    
    # 2. 递归移除所有扩展属性
    echo -e "${BLUE}[2/3] 清理扩展属性...${NC}"
    xattr -cr "$app_path" 2>/dev/null || true
    echo -e "${GREEN}  ✓ 扩展属性已清理${NC}"
    
    # 3. 添加执行权限
    echo -e "${BLUE}[3/3] 设置执行权限...${NC}"
    chmod -R 755 "$app_path"
    chmod +x "$app_path/Contents/MacOS/"* 2>/dev/null || true
    echo -e "${GREEN}  ✓ 执行权限已设置${NC}"
    
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  修复完成！${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo -e "现在可以尝试打开应用了。"
    echo ""
    echo -e "${YELLOW}如果仍然无法打开，请尝试：${NC}"
    echo -e "  1. 右键点击应用 → 选择「打开」"
    echo -e "  2. 在弹出的对话框中点击「打开」"
    echo ""
    echo -e "${YELLOW}或者在系统设置中允许：${NC}"
    echo -e "  系统设置 → 隐私与安全性 → 安全性 → 点击「仍要打开」"
    echo ""
}

# 主逻辑
main() {
    # 如果提供了参数，使用参数作为路径
    if [ -n "$1" ]; then
        if [ -d "$1" ]; then
            fix_app "$1"
        else
            echo -e "${RED}错误: 找不到应用: $1${NC}"
            exit 1
        fi
    else
        # 自动查找应用
        echo -e "${YELLOW}正在查找 ${APP_NAME}.app ...${NC}"
        
        APP_PATH=$(find_app)
        
        if [ -n "$APP_PATH" ]; then
            echo -e "${GREEN}✓ 找到应用: ${APP_PATH}${NC}"
            echo ""
            fix_app "$APP_PATH"
        else
            echo -e "${RED}错误: 找不到 ${APP_NAME}.app${NC}"
            echo ""
            echo "请将应用拖到此脚本上运行，或指定路径："
            echo "  ./fix-mac.sh /path/to/${APP_NAME}.app"
            echo ""
            echo "或者将应用移动到以下位置之一："
            echo "  - /Applications/"
            echo "  - ~/Applications/"
            echo "  - ~/Desktop/"
            echo "  - ~/Downloads/"
            exit 1
        fi
    fi
}

main "$@"

