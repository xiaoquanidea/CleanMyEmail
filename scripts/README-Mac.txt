CleanMyEmail - macOS 安装说明
================================

1. 将 CleanMyEmail.app 拖入「应用程序」文件夹

2. 双击打开应用

如果提示「无法打开」或「已损坏」：
----------------------------------------
在终端中执行修复脚本：

  ./fix-mac.sh

或者手动执行以下命令：

  sudo xattr -rd com.apple.quarantine /Applications/CleanMyEmail.app

然后重新打开应用即可。

