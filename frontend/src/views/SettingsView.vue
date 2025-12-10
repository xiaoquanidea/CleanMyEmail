<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { NLayout, NLayoutContent, NCard, NForm, NFormItem, NInput, NInputNumber, NSelect, NSwitch, NButton, NSpace, NAlert, NIcon, NPageHeader, NPopconfirm, NDescriptions, NDescriptionsItem, NScrollbar } from 'naive-ui'
import { ArrowBack, CheckmarkCircle, TrashOutline, RefreshOutline } from '@vicons/ionicons5'
import { GetProxySettings, SaveProxySettings, TestProxy, GetAppVersion, ClearAllCleanHistory } from '../../wailsjs/go/main/App'
import { useFolderStore } from '../stores/folder'

const router = useRouter()
const message = useMessage()
const folderStore = useFolderStore()

const loading = ref(false)
const testing = ref(false)
const testResult = ref<{ success: boolean; message: string } | null>(null)

// 版本信息
const appVersion = ref({ version: '', buildTime: '' })

type ProxyType = 'none' | 'socks5' | 'http'

const proxySettings = ref({
  type: 'none' as ProxyType,
  host: '127.0.0.1',
  port: 7891,
  enabled: false
})

const proxyTypeOptions = [
  { label: '无代理（直连）', value: 'none' },
  { label: 'SOCKS5 代理', value: 'socks5' },
  { label: 'HTTP 代理（暂不支持IMAP）', value: 'http', disabled: true }
]

const loadSettings = async () => {
  try {
    const settings = await GetProxySettings()
    if (settings) {
      proxySettings.value = {
        type: (settings.type || 'none') as ProxyType,
        host: settings.host || '127.0.0.1',
        port: settings.port || 7891,
        enabled: settings.enabled || false
      }
    }
  } catch (e) {
    console.error('加载设置失败:', e)
  }
}

const loadVersion = async () => {
  try {
    const version = await GetAppVersion()
    if (version) {
      appVersion.value = version
    }
  } catch (e) {
    console.error('获取版本信息失败:', e)
  }
}

const handleSave = async () => {
  loading.value = true
  try {
    await SaveProxySettings(proxySettings.value)
    testResult.value = { success: true, message: '设置已保存' }
  } catch (e: any) {
    testResult.value = { success: false, message: '保存失败: ' + e.message }
  } finally {
    loading.value = false
  }
}

const handleTest = async () => {
  testing.value = true
  testResult.value = null
  try {
    await TestProxy(proxySettings.value)
    testResult.value = { success: true, message: '代理连接测试成功！' }
  } catch (e: any) {
    testResult.value = { success: false, message: '测试失败: ' + e.message }
  } finally {
    testing.value = false
  }
}

const handleClearFolderCache = () => {
  folderStore.clearAllCache()
  message.success('文件夹缓存已清除')
}

const handleClearHistory = async () => {
  try {
    await ClearAllCleanHistory()
    message.success('清理历史已清空')
  } catch (e: any) {
    message.error('清空失败: ' + e)
  }
}

const handleBack = () => {
  router.push('/')
}

onMounted(() => {
  loadSettings()
  loadVersion()
})
</script>

<template>
  <n-layout class="layout">
    <n-layout-content content-style="padding: 24px;">
      <n-page-header @back="handleBack" title="设置" subtitle="配置应用全局设置">
        <template #back>
          <n-icon><ArrowBack /></n-icon>
        </template>
      </n-page-header>

      <n-card title="代理设置" style="margin-top: 16px;">
        <template #header-extra>
          <n-switch v-model:value="proxySettings.enabled" :disabled="proxySettings.type === 'none'">
            <template #checked>已启用</template>
            <template #unchecked>已禁用</template>
          </n-switch>
        </template>

        <n-form label-placement="left" label-width="100">
          <n-form-item label="代理类型">
            <n-select v-model:value="proxySettings.type" :options="proxyTypeOptions" style="width: 250px;" />
          </n-form-item>

          <template v-if="proxySettings.type !== 'none'">
            <n-form-item label="代理地址">
              <n-input v-model:value="proxySettings.host" placeholder="127.0.0.1" style="width: 200px;" />
            </n-form-item>

            <n-form-item label="代理端口">
              <n-input-number v-model:value="proxySettings.port" :min="1" :max="65535" style="width: 150px;" />
            </n-form-item>
          </template>

          <n-form-item label=" ">
            <n-space>
              <n-button type="primary" :loading="loading" @click="handleSave">保存设置</n-button>
              <n-button :loading="testing" :disabled="proxySettings.type === 'none'" @click="handleTest">
                测试连接
              </n-button>
            </n-space>
          </n-form-item>
        </n-form>

        <n-alert v-if="testResult" :type="testResult.success ? 'success' : 'error'" style="margin-top: 16px;">
          {{ testResult.message }}
        </n-alert>

        <n-alert type="info" style="margin-top: 16px;">
          <p><strong>提示：</strong></p>
          <ul style="margin: 8px 0 0 16px; padding: 0;">
            <li>如果你使用 Clash/V2Ray 等代理软件，通常 SOCKS5 端口是 7891 或 1080</li>
            <li>如果使用 TUN 模式（虚拟网卡），可以选择"无代理"，流量会自动走代理</li>
            <li>Gmail 等国外邮箱需要代理才能连接</li>
          </ul>
        </n-alert>
      </n-card>

      <!-- 数据管理 -->
      <n-card title="数据管理" style="margin-top: 16px;">
        <n-space vertical>
          <n-space align="center">
            <span style="width: 120px;">文件夹缓存：</span>
            <n-button @click="handleClearFolderCache">
              <template #icon><n-icon><RefreshOutline /></n-icon></template>
              清除缓存
            </n-button>
            <span style="color: #999; font-size: 12px;">清除后下次加载文件夹将重新获取</span>
          </n-space>
          <n-space align="center">
            <span style="width: 120px;">清理历史：</span>
            <n-popconfirm @positive-click="handleClearHistory">
              <template #trigger>
                <n-button type="error">
                  <template #icon><n-icon><TrashOutline /></n-icon></template>
                  清空历史
                </n-button>
              </template>
              确定清空所有清理历史记录？
            </n-popconfirm>
          </n-space>
        </n-space>
      </n-card>

      <!-- 关于 -->
      <n-card title="关于" style="margin-top: 16px;">
        <n-descriptions :column="1" label-placement="left" :label-style="{ width: '100px' }">
          <n-descriptions-item label="应用名称">CleanMyEmail</n-descriptions-item>
          <n-descriptions-item label="版本">v{{ appVersion.version || '1.0.0' }}</n-descriptions-item>
          <n-descriptions-item label="构建时间">{{ appVersion.buildTime || 'unknown' }}</n-descriptions-item>
          <n-descriptions-item label="作者">hutiquan</n-descriptions-item>
          <n-descriptions-item label="邮箱">xiaoqunidea@163.com</n-descriptions-item>
        </n-descriptions>
        <n-alert type="default" style="margin-top: 16px;">
          CleanMyEmail 是一款邮箱清理工具，帮助你批量删除旧邮件，释放邮箱空间。
        </n-alert>
      </n-card>
    </n-layout-content>
  </n-layout>
</template>

<style scoped>
.layout {
  height: 100vh;
  background: #f5f5f5;
}

.layout :deep(.n-page-header) {
  -webkit-app-region: drag;
  padding: 8px 0;
}

.layout :deep(.n-page-header .n-page-header-header),
.layout :deep(.n-page-header button) {
  -webkit-app-region: no-drag;
}
</style>

