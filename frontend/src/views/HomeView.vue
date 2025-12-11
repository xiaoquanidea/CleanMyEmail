<script lang="ts" setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { NLayout, NLayoutSider, NLayoutContent, NButton, NEmpty, NSpin, NCard, NTag, NSpace, NPopconfirm, NIcon, NTooltip } from 'naive-ui'
import { Add, Trash, Mail, RefreshOutline, Settings, TimeOutline, KeyOutline, WarningOutline, SparklesOutline } from '@vicons/ionicons5'
import { useAccountStore } from '../stores/account'
import { StartOAuth2Reauth, WaitOAuth2Callback, CancelOAuth2Auth, GetVersion } from '../../wailsjs/go/main/App'

// 导入邮箱图标
import gmailIcon from '../assets/icons/gmail.svg'
import outlookIcon from '../assets/icons/outlook.svg'
import qqIcon from '../assets/icons/qq.svg'
import neteaseIcon from '../assets/icons/netease.ico'
import aliyunIcon from '../assets/icons/aliyun.png'
import otherIcon from '../assets/icons/other.svg'

// 图标映射
const iconMap: Record<string, string> = {
  gmail: gmailIcon,
  outlook: outlookIcon,
  qq: qqIcon,
  netease: neteaseIcon,
  aliyun: aliyunIcon,
  other: otherIcon
}

// 根据厂商类型获取图标
const getVendorIcon = (vendor: string) => {
  const vendorIconMap: Record<string, string> = {
    'gmail': 'gmail',
    'outlook': 'outlook',
    'qq': 'qq',
    '163-personal': 'netease',
    '163-enterprise': 'netease',
    '126': 'netease',
    'aliyun': 'aliyun',
    'other': 'other'
  }
  const iconKey = vendorIconMap[vendor] || 'other'
  return iconMap[iconKey] || otherIcon
}

const router = useRouter()
const message = useMessage()
const accountStore = useAccountStore()
const reauthorizing = ref<number | null>(null)
const currentReauthState = ref('')  // 保存当前重新授权的 state
const appVersion = ref('')

const statusTagType = (status: string) => {
  switch (status) {
    case 'active': return 'success'
    case 'disconnected': return 'error'
    case 'auth_required': return 'warning'
    default: return 'default'
  }
}

const statusText = (status: string) => {
  switch (status) {
    case 'active': return '已连接'
    case 'disconnected': return '已断开'
    case 'auth_required': return '需授权'
    default: return status
  }
}

// 判断是否需要显示重新授权按钮
const needsReauth = (account: any) => {
  return account.authType?.startsWith('oauth2') &&
    (account.status === 'auth_required' || account.tokenWarning)
}

// 判断是否是 OAuth2 账号
const isOAuth2Account = (account: any) => {
  return account.authType?.startsWith('oauth2')
}

// 重新授权
const handleReauthorize = async (account: any) => {
  if (!isOAuth2Account(account)) {
    message.error('该账号不支持 OAuth2 授权')
    return
  }

  reauthorizing.value = account.id
  try {
    // 使用专门的重新授权方法，传入账号ID
    const result = await StartOAuth2Reauth(account.id)
    currentReauthState.value = result.state
    // 重新授权时 email 参数会被忽略，后端使用 session 中保存的
    await WaitOAuth2Callback(result.state, '')
    message.success('重新授权成功！')
    accountStore.fetchAccounts()
  } catch (error: any) {
    message.error(`授权失败: ${error}`)
  } finally {
    reauthorizing.value = null
    currentReauthState.value = ''
  }
}

// 取消重新授权
const handleCancelReauth = () => {
  if (currentReauthState.value) {
    CancelOAuth2Auth(currentReauthState.value)
  }
  reauthorizing.value = null
  currentReauthState.value = ''
}

const handleAddAccount = () => {
  router.push('/account/add')
}

const handleSelectAccount = (id: number) => {
  accountStore.setCurrentAccount(id)
  router.push(`/clean/${id}`)
}

const handleDeleteAccount = async (id: number) => {
  await accountStore.removeAccount(id)
}

const handleSettings = () => {
  router.push('/settings')
}

const handleHistory = () => {
  router.push('/history')
}

onMounted(async () => {
  accountStore.fetchAccounts()
  try {
    appVersion.value = await GetVersion()
  } catch {
    appVersion.value = ''
  }
})
</script>

<template>
  <n-layout class="layout" has-sider>
    <!-- 左侧账号列表 -->
    <n-layout-sider
      bordered
      :width="280"
      content-style="padding: 16px;"
      class="sider"
    >
      <div class="sider-header">
        <div class="header-top">
          <div class="brand">
            <div class="logo-icon">
              <n-icon size="20" color="#fff"><SparklesOutline /></n-icon>
            </div>
            <div class="brand-info">
              <h2 class="title">CleanMyEmail</h2>
              <span v-if="appVersion" class="version">{{ appVersion }}</span>
            </div>
          </div>
          <n-space :size="4">
            <n-tooltip trigger="hover">
              <template #trigger>
                <n-button size="small" quaternary class="header-btn" @click="handleHistory">
                  <template #icon>
                    <n-icon size="18"><TimeOutline /></n-icon>
                  </template>
                </n-button>
              </template>
              清理历史
            </n-tooltip>
            <n-tooltip trigger="hover">
              <template #trigger>
                <n-button size="small" quaternary class="header-btn" @click="handleSettings">
                  <template #icon>
                    <n-icon size="18"><Settings /></n-icon>
                  </template>
                </n-button>
              </template>
              设置
            </n-tooltip>
          </n-space>
        </div>
        <n-button type="primary" size="small" block @click="handleAddAccount" class="add-btn">
          <template #icon>
            <n-icon><Add /></n-icon>
          </template>
          添加账号
        </n-button>
      </div>

      <n-spin :show="accountStore.loading">
        <div class="account-list">
          <template v-if="accountStore.accounts.length > 0">
            <n-card
              v-for="account in accountStore.accounts"
              :key="account.id"
              size="small"
              hoverable
              class="account-card"
              :class="{ active: accountStore.currentAccountId === account.id, warning: account.tokenWarning }"
              @click="handleSelectAccount(account.id)"
            >
              <div class="account-row">
                <img :src="getVendorIcon(account.vendor)" :alt="account.vendor" class="vendor-icon" />
                <div class="account-info">
                  <div class="account-email">{{ account.email }}</div>
                  <n-space :size="4" align="center">
                    <n-tag :type="statusTagType(account.status)" size="tiny">
                      {{ statusText(account.status) }}
                    </n-tag>
                    <!-- Token 警告提示 -->
                    <n-tooltip v-if="account.tokenWarning" trigger="hover">
                      <template #trigger>
                        <n-icon color="#f0a020" size="14"><WarningOutline /></n-icon>
                      </template>
                      {{ account.tokenWarning }}
                    </n-tooltip>
                  </n-space>
                </div>
                <div class="account-actions" @click.stop>
                  <n-space :size="4">
                    <!-- 重新授权按钮 -->
                    <n-tooltip v-if="needsReauth(account)" trigger="hover">
                      <template #trigger>
                        <n-button
                          text
                          type="warning"
                          size="small"
                          :loading="reauthorizing === account.id"
                          @click="handleReauthorize(account)"
                        >
                          <template #icon>
                            <n-icon><KeyOutline /></n-icon>
                          </template>
                        </n-button>
                      </template>
                      重新授权
                    </n-tooltip>
                    <!-- 删除按钮 -->
                    <n-popconfirm @positive-click="handleDeleteAccount(account.id)">
                      <template #trigger>
                        <n-button text type="error" size="small">
                          <template #icon>
                            <n-icon><Trash /></n-icon>
                          </template>
                        </n-button>
                      </template>
                      确定删除此账号吗？
                    </n-popconfirm>
                  </n-space>
                </div>
              </div>
            </n-card>
          </template>
          <n-empty v-else description="暂无账号，请添加" />
        </div>
      </n-spin>
    </n-layout-sider>

    <!-- 右侧内容区 -->
    <n-layout-content content-style="padding: 24px;" class="content">
      <div class="welcome">
        <!-- 动画邮件图标 -->
        <div class="icon-container">
          <div class="pulse-ring"></div>
          <div class="pulse-ring delay-1"></div>
          <div class="pulse-ring delay-2"></div>
          <div class="icon-wrapper">
            <n-icon size="64" color="#fff">
              <Mail />
            </n-icon>
          </div>
          <!-- 飞舞的小邮件 -->
          <div class="floating-mail mail-1">✉</div>
          <div class="floating-mail mail-2">📧</div>
          <div class="floating-mail mail-3">📨</div>
        </div>

        <h1 class="title-animate">
          <span class="title-text">欢迎使用</span>
          <span class="brand-text">CleanMyEmail</span>
        </h1>
        <p class="subtitle-animate">选择左侧账号开始清理邮件，或添加新账号</p>

        <!-- 功能亮点 -->
        <div class="features">
          <div class="feature-item">
            <span class="feature-icon">🚀</span>
            <span class="feature-title">高性能并发</span>
            <span class="feature-desc">连接池 + 多协程并行处理</span>
          </div>
          <div class="feature-item">
            <span class="feature-icon">🔒</span>
            <span class="feature-title">隐私安全</span>
            <span class="feature-desc">数据本地处理，不上传云端</span>
          </div>
          <div class="feature-item">
            <span class="feature-icon">🎯</span>
            <span class="feature-title">精准筛选</span>
            <span class="feature-desc">按日期/发件人/主题/大小过滤</span>
          </div>
          <div class="feature-item">
            <span class="feature-icon">🔑</span>
            <span class="feature-title">多种认证</span>
            <span class="feature-desc">支持密码和 OAuth2 授权</span>
          </div>
        </div>

        <n-button v-if="accountStore.accounts.length === 0" type="primary" size="large" class="add-btn-animate" @click="handleAddAccount">
          <template #icon>
            <n-icon><Add /></n-icon>
          </template>
          添加第一个账号
        </n-button>
      </div>
      <div class="copyright">
        <p>© 2025 hutiquan | 海管家 · 订舱平台 | xiaoquanidea@163.com</p>
        <p class="disclaimer">本项目纯用爱发电，免费开源。邮件删除不可恢复，使用者自行承担风险。</p>
        <p class="disclaimer">所有数据都在用户端处理和存储，不上传云端</p>
      </div>
    </n-layout-content>
  </n-layout>
</template>

<style scoped>
.layout {
  height: 100vh;
}

.sider {
  background: #fafafa;
}

.sider-header {
  margin-bottom: 16px;
  padding-bottom: 16px;
  border-bottom: 1px solid #e8e8e8;
  background: linear-gradient(180deg, #f8fdf9 0%, #fafafa 100%);
  margin: -16px -16px 16px -16px;
  padding: 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}

.header-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  -webkit-app-region: drag;
}

.header-top :deep(button) {
  -webkit-app-region: no-drag;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.logo-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: linear-gradient(135deg, #18a058 0%, #36ad6a 50%, #63e2b7 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 8px rgba(24, 160, 88, 0.3);
  animation: logoShine 3s ease-in-out infinite;
}

@keyframes logoShine {
  0%, 100% { box-shadow: 0 2px 8px rgba(24, 160, 88, 0.3); }
  50% { box-shadow: 0 4px 16px rgba(24, 160, 88, 0.5); }
}

.brand-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  background: linear-gradient(135deg, #18a058, #36ad6a);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  cursor: default;
  line-height: 1.2;
}

.version {
  font-size: 11px;
  color: #999;
  font-weight: 400;
}

.header-btn {
  color: #666;
  transition: all 0.2s;
}

.header-btn:hover {
  color: #18a058;
  background: rgba(24, 160, 88, 0.1);
}

.add-btn {
  width: 100%;
  animation: btnReady 2s ease-in-out infinite;
}

@keyframes btnReady {
  0%, 100% { box-shadow: 0 2px 4px rgba(24, 160, 88, 0.2); }
  50% { box-shadow: 0 4px 12px rgba(24, 160, 88, 0.35); }
}

.account-list {
  min-height: 200px;
}

.account-card {
  margin-bottom: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.account-card:hover {
  border-color: #18a058;
}

.account-card.warning {
  border-color: #f0a020;
  background: #fffbe6;
}

.account-card.active {
  border-color: #18a058;
  background: #f0faf4;
}

.account-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.vendor-icon {
  width: 28px;
  height: 28px;
  object-fit: contain;
  flex-shrink: 0;
}

.account-info {
  flex: 1;
  min-width: 0;
}

.account-email {
  font-weight: 500;
  margin-bottom: 4px;
  word-break: break-all;
  font-size: 13px;
}

.account-actions {
  flex-shrink: 0;
}

.content {
  background: #fff;
}

.welcome {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  text-align: center;
  color: #666;
}

/* 图标容器 */
.icon-container {
  position: relative;
  width: 120px;
  height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 16px;
}

.icon-wrapper {
  width: 80px;
  height: 80px;
  border-radius: 50%;
  background: linear-gradient(135deg, #18a058 0%, #36ad6a 50%, #63e2b7 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 8px 32px rgba(24, 160, 88, 0.35);
  animation: iconFloat 3s ease-in-out infinite;
  z-index: 2;
}

@keyframes iconFloat {
  0%, 100% { transform: translateY(0) scale(1); }
  50% { transform: translateY(-8px) scale(1.05); }
}

/* 脉冲光环 */
.pulse-ring {
  position: absolute;
  width: 80px;
  height: 80px;
  border-radius: 50%;
  border: 2px solid #18a058;
  animation: pulseRing 2s ease-out infinite;
}

.pulse-ring.delay-1 { animation-delay: 0.6s; }
.pulse-ring.delay-2 { animation-delay: 1.2s; }

@keyframes pulseRing {
  0% { transform: scale(1); opacity: 0.6; }
  100% { transform: scale(2); opacity: 0; }
}

/* 飞舞的小邮件 */
.floating-mail {
  position: absolute;
  font-size: 20px;
  animation: floatMail 4s ease-in-out infinite;
  opacity: 0.7;
}

.mail-1 { top: 0; left: 10px; animation-delay: 0s; }
.mail-2 { top: 20px; right: 0; animation-delay: 1.3s; }
.mail-3 { bottom: 10px; left: 0; animation-delay: 2.6s; }

@keyframes floatMail {
  0%, 100% { transform: translate(0, 0) rotate(0deg); opacity: 0.7; }
  25% { transform: translate(5px, -10px) rotate(10deg); opacity: 1; }
  50% { transform: translate(-5px, -5px) rotate(-5deg); opacity: 0.8; }
  75% { transform: translate(3px, 5px) rotate(5deg); opacity: 0.9; }
}

/* 标题动画 */
.title-animate {
  margin: 8px 0;
  animation: fadeInUp 0.8s ease-out;
}

.title-text {
  color: #333;
  font-size: 28px;
  font-weight: 400;
}

.brand-text {
  background: linear-gradient(135deg, #18a058, #36ad6a, #63e2b7);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  font-size: 32px;
  font-weight: 700;
  margin-left: 8px;
}

.subtitle-animate {
  color: #888;
  font-size: 15px;
  margin-bottom: 24px;
  animation: fadeInUp 0.8s ease-out 0.2s both;
}

@keyframes fadeInUp {
  from { opacity: 0; transform: translateY(20px); }
  to { opacity: 1; transform: translateY(0); }
}

/* 功能亮点 */
.features {
  display: flex;
  gap: 32px;
  margin-bottom: 32px;
  animation: fadeInUp 0.8s ease-out 0.4s both;
}

.feature-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 20px 24px;
  border-radius: 16px;
  background: linear-gradient(135deg, #f8fdf9 0%, #f0faf3 100%);
  border: 1px solid #e8f5ec;
  transition: all 0.3s ease;
  min-width: 160px;
}

.feature-item:hover {
  transform: translateY(-6px);
  box-shadow: 0 12px 32px rgba(24, 160, 88, 0.18);
  border-color: #c5e8d2;
  background: linear-gradient(135deg, #f0faf3 0%, #e8f5ec 100%);
}

.feature-icon {
  font-size: 32px;
  margin-bottom: 4px;
}

.feature-title {
  color: #333;
  font-size: 15px;
  font-weight: 600;
}

.feature-desc {
  color: #888;
  font-size: 12px;
  font-weight: 400;
}

/* 按钮动画 */
.add-btn-animate {
  animation: fadeInUp 0.8s ease-out 0.6s both, btnPulse 2s ease-in-out 1.5s infinite;
}

@keyframes btnPulse {
  0%, 100% { box-shadow: 0 2px 8px rgba(24, 160, 88, 0.3); }
  50% { box-shadow: 0 4px 20px rgba(24, 160, 88, 0.5); }
}

.copyright {
  position: absolute;
  bottom: 16px;
  left: 0;
  right: 0;
  text-align: center;
  color: #999;
  font-size: 12px;
}

.copyright p {
  margin: 0;
}

.copyright .disclaimer {
  margin-top: 4px;
  font-size: 11px;
  color: #bbb;
}
</style>
