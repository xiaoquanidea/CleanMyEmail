# Privacy Policy / 隐私政策

## English

### Introduction
CleanMyEmail is a desktop email management application that helps users clean and organize their mailboxes. This privacy policy explains how we handle your data.

### Data Collection
CleanMyEmail does **not** collect, store, or transmit any user data to external servers. All operations are performed entirely on your local device.

### Email Access
- The app accesses your email through the IMAP protocol on your local device
- All actions are initiated by you, the user
- The app only reads email metadata (sender, subject, date, size) for display and filtering purposes
- Email deletion is only performed when you explicitly request it
- No email content is read, analyzed, or stored by the application

### Supported Email Services
This application supports the following email services via OAuth2 authentication:

**Google Gmail**
- Uses Google OAuth2 for secure authentication
- Requires `https://mail.google.com/` scope for full mailbox access (IMAP read/delete, SMTP send)
- You can revoke access at any time: [Google Account Permissions](https://myaccount.google.com/permissions)
- Refer to [Google's Privacy Policy](https://policies.google.com/privacy)

**Microsoft Outlook**
- Uses Microsoft OAuth2 with PKCE for secure authentication
- Requires `IMAP.AccessAsUser.All` scope for IMAP access (read and delete emails)
- Requires `SMTP.Send` scope for sending emails (future feature)
- You can revoke access at any time: [Microsoft Account Permissions](https://account.live.com/consent/Manage)
- Refer to [Microsoft's Privacy Statement](https://privacy.microsoft.com/privacystatement)

### Data Storage
- Email account credentials are stored locally on your device only
- OAuth2 tokens are stored locally and used only for authentication
- All data is stored in a local SQLite database on your device
- No data is transmitted to any external servers

### Data Security
- All communications with email servers use encrypted connections (TLS/SSL)
- OAuth2 tokens are stored securely on your local device
- The application does not have access to your email password when using OAuth2

### Your Rights
- You can delete all stored data by removing the application
- You can revoke OAuth2 access at any time through your Google or Microsoft account settings
- You have full control over which emails to delete

### Contact
If you have any questions about this privacy policy, please open an issue on our [GitHub repository](https://github.com/xiaoquanidea/CleanMyEmail).

---

## 中文

### 简介
CleanMyEmail 是一款桌面邮箱管理应用，帮助用户清理和整理邮箱。本隐私政策说明我们如何处理您的数据。

### 数据收集
CleanMyEmail **不会**收集、存储或传输任何用户数据到外部服务器。所有操作完全在您的本地设备上执行。

### 邮箱访问
- 本应用通过 IMAP 协议在您的本地设备上访问邮箱
- 所有操作由您主动发起
- 仅读取邮件元数据（发件人、主题、日期、大小）用于展示和筛选
- 仅在您明确请求时才执行邮件删除操作
- 本应用不会读取、分析或存储任何邮件内容

### 支持的邮箱服务
本应用通过 OAuth2 认证支持以下邮箱服务：

**Google Gmail**
- 使用 Google OAuth2 进行安全认证
- 需要 `https://mail.google.com/` 权限用于完整邮箱访问（IMAP 读取/删除、SMTP 发送）
- 您可以随时撤销授权：[Google 账户权限](https://myaccount.google.com/permissions)
- 请参阅 [Google 隐私政策](https://policies.google.com/privacy)

**Microsoft Outlook**
- 使用 Microsoft OAuth2 + PKCE 进行安全认证
- 需要 `IMAP.AccessAsUser.All` 权限用于 IMAP 访问（读取和删除邮件）
- 需要 `SMTP.Send` 权限用于发送邮件（未来功能）
- 您可以随时撤销授权：[Microsoft 账户权限](https://account.live.com/consent/Manage)
- 请参阅 [Microsoft 隐私声明](https://privacy.microsoft.com/privacystatement)

### 数据存储
- 邮箱账户凭据仅存储在您的本地设备上
- OAuth2 令牌仅存储在本地，仅用于身份验证
- 所有数据存储在您设备上的本地 SQLite 数据库中
- 不会向任何外部服务器传输数据

### 数据安全
- 与邮件服务器的所有通信均使用加密连接（TLS/SSL）
- OAuth2 令牌安全存储在您的本地设备上
- 使用 OAuth2 时，本应用无法访问您的邮箱密码

### 您的权利
- 您可以通过删除应用来删除所有存储的数据
- 您可以随时通过 Google 或 Microsoft 账户设置撤销 OAuth2 授权
- 您完全控制要删除哪些邮件

### 联系方式
如果您对本隐私政策有任何疑问，请在我们的 [GitHub 仓库](https://github.com/xiaoquanidea/CleanMyEmail) 提交 issue。

---

Last Updated / 最后更新：2025-12

