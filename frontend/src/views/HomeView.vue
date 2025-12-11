<script lang="ts" setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { NLayout, NLayoutSider, NLayoutContent, NButton, NEmpty, NSpin, NCard, NTag, NSpace, NPopconfirm, NIcon, NTooltip } from 'naive-ui'
import { Add, Trash, Mail, RefreshOutline, Settings, TimeOutline, KeyOutline, WarningOutline } from '@vicons/ionicons5'
import { useAccountStore } from '../stores/account'
import { StartOAuth2Auth, WaitOAuth2Callback, CancelOAuth2Auth } from '../../wailsjs/go/main/App'

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

// 获取厂商名称用于 OAuth2
const getVendorForOAuth2 = (vendor: string) => {
  if (vendor === 'gmail') return 'gmail'
  if (vendor === 'outlook') return 'outlook'
  return ''
}

// 重新授权
const handleReauthorize = async (account: any) => {
  const vendor = getVendorForOAuth2(account.vendor)
  if (!vendor) {
    message.error('该账号不支持 OAuth2 授权')
    return
  }

  reauthorizing.value = account.id
  try {
    await StartOAuth2Auth(vendor)
    await WaitOAuth2Callback(vendor, account.email)
    message.success('重新授权成功！')
    accountStore.fetchAccounts()
  } catch (error: any) {
    message.error(`授权失败: ${error}`)
  } finally {
    reauthorizing.value = null
  }
}

// 取消重新授权
const handleCancelReauth = () => {
  CancelOAuth2Auth()
  reauthorizing.value = null
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

onMounted(() => {
  accountStore.fetchAccounts()
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
          <h2 class="title">CleanMyEmail</h2>
          <n-space :size="4">
            <n-button size="small" quaternary @click="handleHistory" title="清理历史">
              <template #icon>
                <n-icon size="18"><TimeOutline /></n-icon>
              </template>
            </n-button>
            <n-button size="small" quaternary @click="handleSettings" title="设置">
              <template #icon>
                <n-icon size="18"><Settings /></n-icon>
              </template>
            </n-button>
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
        <n-icon size="80" color="#18a058">
          <Mail />
        </n-icon>
        <h1>欢迎使用 CleanMyEmail</h1>
        <p>选择左侧账号开始清理邮件，或添加新账号</p>
        <n-button v-if="accountStore.accounts.length === 0" type="primary" size="large" @click="handleAddAccount">
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
  border-bottom: 1px solid #eee;
}

.header-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  -webkit-app-region: drag;
  padding: 4px 0;
}

.header-top :deep(button) {
  -webkit-app-region: no-drag;
}

.title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #333;
  cursor: default;
}

.settings-btn {
  color: #666;
}

.add-btn {
  width: 100%;
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

.welcome h1 {
  margin: 24px 0 8px;
  color: #333;
}

.welcome p {
  margin-bottom: 24px;
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
