# CleanMyEmail

一款简洁高效的邮箱清理工具，帮助你批量删除旧邮件，释放邮箱空间。

## ✨ 功能特性

- **多邮箱支持**: 163企业/个人、126、QQ、阿里云、Gmail、Outlook
- **多种认证方式**: 密码认证、OAuth2 授权（Gmail/Outlook）
- **灵活的筛选条件**:
  - 日期范围筛选
  - 发件人筛选（支持多个，OR 逻辑）
  - 主题关键词筛选
  - 邮件大小筛选
  - 已读/未读状态筛选
- **安全可靠**:
  - 预览模式：清理前可预览将被删除的邮件
  - 所有数据本地存储，不上传云端
  - 支持 SOCKS5 代理
- **高性能**: 批量处理、并发删除、自动重试

## 📸 截图

（待添加）

## 🚀 快速开始

### 下载安装

从 [Releases](https://github.com/xiaoquanidea/CleanMyEmail/releases) 页面下载对应平台的安装包：

- **macOS**: `CleanMyEmail-x.x.x-darwin-arm64.zip` (Apple Silicon) 或 `CleanMyEmail-x.x.x-darwin-amd64.zip` (Intel)
- **Windows**: `CleanMyEmail-x.x.x-windows-amd64.zip`

### macOS 用户注意

首次运行可能提示"无法打开"，请执行以下步骤：
1. 解压后，右键点击 `fix-mac.sh`，选择"打开"
2. 或在终端执行: `xattr -cr CleanMyEmail.app`

### 添加邮箱账号

1. 点击"添加账号"
2. 选择邮箱类型
3. 输入邮箱地址
4. 根据邮箱类型选择认证方式：
   - **密码认证**: 输入邮箱密码或授权码
   - **OAuth2**: 点击浏览器授权（Gmail/Outlook）

### 清理邮件

1. 选择要清理的账号
2. 选择文件夹（收件箱、已发送等）
3. 设置筛选条件
4. 点击"预览"确认要删除的邮件
5. 点击"开始清理"执行删除

## 🔧 开发

### 环境要求

- Go 1.21+
- Node.js 18+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

### 本地开发

```bash
# 安装依赖
wails dev
```

### 构建发布版本

```bash
# 使用构建脚本（推荐）
./scripts/build.sh darwin-arm64 1.0.0

# 或直接使用 wails
wails build -ldflags "-X 'main.Version=1.0.0' -X 'main.BuildTime=$(date +%Y-%m-%d\ %H:%M:%S)'"
```

## 🛡️ 隐私声明

- 所有数据都在本地处理和存储
- 不收集任何用户信息
- 不上传任何数据到云端
- OAuth2 认证使用官方授权流程

## ⚠️ 免责声明

- 邮件删除后**不可恢复**，请谨慎操作
- 建议先使用预览功能确认要删除的邮件
- 使用本工具产生的任何后果由用户自行承担

## 📄 许可证

MIT License

## 👨‍💻 作者

- hutiquan
- Email: xiaoquanidea@163.com

---

**本项目纯用爱发电，免费开源。如果对你有帮助，欢迎 Star ⭐**
