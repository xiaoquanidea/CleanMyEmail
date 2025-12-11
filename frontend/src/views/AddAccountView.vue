<script lang="ts" setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { NCard, NForm, NFormItem, NInput, NButton, NSpace, NGrid, NGridItem, NIcon, NAlert, NModal, NSpin } from 'naive-ui'
import { ArrowBack, CheckmarkCircle, Settings } from '@vicons/ionicons5'
import { GetVendorList, TestConnection, CreateAccount, StartOAuth2Auth, WaitOAuth2Callback, CancelOAuth2Auth, GetOAuth2Config, SaveOAuth2Config } from '../../wailsjs/go/main/App'

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

const getVendorIcon = (icon: string) => {
  return iconMap[icon] || otherIcon
}

interface VendorInfo {
  vendor: string
  name: string
  icon: string
  imapServer: string
  supportsOAuth: boolean
}

const router = useRouter()
const message = useMessage()

const vendors = ref<VendorInfo[]>([])
const loading = ref(false)
const testing = ref(false)
const testSuccess = ref(false)

// OAuth2 相关状态
const oauth2Waiting = ref(false)
const showOAuth2ConfigModal = ref(false)
const oauth2ConfigExists = ref(false)
const oauth2Config = ref({
  clientId: '',
  clientSecret: ''
})
// 保存当前 OAuth2 会话的 state
const currentOAuth2State = ref('')

const formData = ref({
  vendor: '',
  email: '',
  password: '',
  authType: 'password',
  imapServer: ''
})

const selectedVendor = computed(() => {
  return vendors.value.find(v => v.vendor === formData.value.vendor)
})

// 是否使用 OAuth2 模式
const isOAuth2Mode = computed(() => {
  return selectedVendor.value?.supportsOAuth === true
})

const handleVendorSelect = async (vendor: string) => {
  formData.value.vendor = vendor
  const v = vendors.value.find(item => item.vendor === vendor)
  if (v) {
    formData.value.imapServer = v.imapServer
  }
  // 切换邮箱类型时清空输入，避免混淆
  formData.value.email = ''
  formData.value.password = ''
  testSuccess.value = false

  // 如果是 OAuth2 厂商，检查是否已配置
  if (v?.supportsOAuth) {
    try {
      const config = await GetOAuth2Config(vendor)
      if (config && config.clientId) {
        oauth2Config.value.clientId = config.clientId
        oauth2Config.value.clientSecret = config.clientSecret || ''
        oauth2ConfigExists.value = true
      } else {
        oauth2Config.value.clientId = ''
        oauth2Config.value.clientSecret = ''
        oauth2ConfigExists.value = false
      }
    } catch {
      oauth2Config.value.clientId = ''
      oauth2Config.value.clientSecret = ''
      oauth2ConfigExists.value = false
    }
  }
}

// OAuth2 配置保存
const handleSaveOAuth2Config = async () => {
  if (!oauth2Config.value.clientId) {
    message.warning('请填写 Client ID')
    return
  }
  // Gmail 需要 Client Secret，Outlook 使用 PKCE 不需要
  if (formData.value.vendor === 'gmail' && !oauth2Config.value.clientSecret) {
    message.warning('Gmail 需要填写 Client Secret')
    return
  }
  try {
    const secret = formData.value.vendor === 'gmail' ? oauth2Config.value.clientSecret : ''
    await SaveOAuth2Config(formData.value.vendor, oauth2Config.value.clientId, secret)
    oauth2ConfigExists.value = true
    showOAuth2ConfigModal.value = false
    message.success('OAuth2 配置已保存')
  } catch (error: any) {
    message.error(`保存失败: ${error}`)
  }
}

// OAuth2 授权
const handleOAuth2Auth = async () => {
  if (!formData.value.email) {
    message.warning('请先填写邮箱地址')
    return
  }

  if (!oauth2ConfigExists.value) {
    showOAuth2ConfigModal.value = true
    return
  }

  oauth2Waiting.value = true
  try {
    // 开始授权流程（会自动打开浏览器）
    const result = await StartOAuth2Auth(formData.value.vendor)
    // 保存 state 用于后续匹配
    currentOAuth2State.value = result.state

    // 等待回调（传入 state 而不是 vendor）
    await WaitOAuth2Callback(result.state, formData.value.email)

    message.success('授权成功，账号已添加！')
    router.push('/')
  } catch (error: any) {
    message.error(`授权失败: ${error}`)
  } finally {
    oauth2Waiting.value = false
    currentOAuth2State.value = ''
  }
}

// 取消 OAuth2 授权
const handleCancelOAuth2 = () => {
  if (currentOAuth2State.value) {
    CancelOAuth2Auth(currentOAuth2State.value)
  }
  oauth2Waiting.value = false
  currentOAuth2State.value = ''
}

// 密码模式：测试连接
const handleTestConnection = async () => {
  if (!formData.value.email || !formData.value.password) {
    message.warning('请填写邮箱地址和授权码')
    return
  }

  testing.value = true
  testSuccess.value = false
  try {
    await TestConnection({
      email: formData.value.email,
      vendor: formData.value.vendor,
      authType: 'password',
      password: formData.value.password,
      imapServer: formData.value.imapServer
    })
    testSuccess.value = true
    message.success('连接测试成功！')
  } catch (error: any) {
    message.error(`连接失败: ${error}`)
  } finally {
    testing.value = false
  }
}

// 密码模式：保存账号
const handleSubmit = async () => {
  if (!testSuccess.value) {
    message.warning('请先测试连接')
    return
  }

  loading.value = true
  try {
    await CreateAccount({
      email: formData.value.email,
      vendor: formData.value.vendor,
      authType: 'password',
      password: formData.value.password,
      imapServer: formData.value.imapServer
    })
    message.success('账号添加成功！')
    router.push('/')
  } catch (error: any) {
    message.error(`添加失败: ${error}`)
  } finally {
    loading.value = false
  }
}

const handleBack = () => {
  router.push('/')
}

onMounted(async () => {
  try {
    vendors.value = await GetVendorList()
  } catch (error) {
    console.error('获取厂商列表失败:', error)
  }
})

onUnmounted(() => {
  // 组件卸载时取消可能正在进行的 OAuth2 授权
  if (oauth2Waiting.value && currentOAuth2State.value) {
    CancelOAuth2Auth(currentOAuth2State.value)
  }
})
</script>

<template>
  <div class="add-account-page">
    <div class="header">
      <n-button text @click="handleBack">
        <template #icon>
          <n-icon><ArrowBack /></n-icon>
        </template>
        返回
      </n-button>
      <h2>添加邮箱账号</h2>
    </div>

    <n-card class="form-card">
      <!-- 选择邮箱类型 -->
      <div class="section">
        <h3>选择邮箱类型</h3>
        <n-grid :cols="4" :x-gap="12" :y-gap="12">
          <n-grid-item v-for="vendor in vendors" :key="vendor.vendor">
            <div
              class="vendor-card"
              :class="{ active: formData.vendor === vendor.vendor }"
              @click="handleVendorSelect(vendor.vendor)"
            >
              <img :src="getVendorIcon(vendor.icon)" :alt="vendor.name" class="vendor-icon" />
              <span class="vendor-name">{{ vendor.name }}</span>
            </div>
          </n-grid-item>
        </n-grid>
      </div>

      <!-- 账号信息表单 -->
      <div v-if="formData.vendor" class="section">
        <h3>账号信息</h3>
        <n-form label-placement="left" label-width="100">
          <n-form-item label="邮箱地址">
            <n-input v-model:value="formData.email" placeholder="example@company.com" />
          </n-form-item>

          <!-- OAuth2 模式 -->
          <template v-if="isOAuth2Mode">
            <n-alert type="info" :show-icon="false" style="margin-bottom: 16px;">
              {{ selectedVendor?.name }} 使用 OAuth2 授权登录，点击下方按钮将打开浏览器进行授权
            </n-alert>

            <n-form-item>
              <n-space vertical size="large" style="width: 100%;">
                <n-button
                  type="primary"
                  size="large"
                  block
                  :loading="oauth2Waiting"
                  :disabled="!formData.email"
                  @click="handleOAuth2Auth"
                >
                  <template #icon>
                    <n-icon><CheckmarkCircle /></n-icon>
                  </template>
                  {{ oauth2Waiting ? '等待授权中...' : '使用浏览器登录授权' }}
                </n-button>
                <n-button v-if="oauth2Waiting" block @click="handleCancelOAuth2">
                  取消授权
                </n-button>
                <n-button quaternary size="small" @click="showOAuth2ConfigModal = true" style="color: #999;">
                  <template #icon>
                    <n-icon><Settings /></n-icon>
                  </template>
                  自定义 OAuth2 配置（高级）
                </n-button>
              </n-space>
            </n-form-item>
          </template>

          <!-- 密码/授权码模式 -->
          <template v-else>
            <n-form-item label="授权码">
              <n-input v-model:value="formData.password" type="password" show-password-on="click" placeholder="请输入授权码或密码" />
            </n-form-item>

            <n-form-item label="IMAP服务器">
              <n-input v-model:value="formData.imapServer" placeholder="imap.example.com:993" />
            </n-form-item>

            <n-alert type="info" :show-icon="false" style="margin-bottom: 16px;">
              提示：请在邮箱网页版设置中开启IMAP服务并获取授权码
            </n-alert>

            <n-form-item>
              <n-space>
                <n-button :loading="testing" @click="handleTestConnection">
                  测试连接
                </n-button>
                <n-button type="primary" :loading="loading" :disabled="!testSuccess" @click="handleSubmit">
                  <template v-if="testSuccess" #icon>
                    <n-icon><CheckmarkCircle /></n-icon>
                  </template>
                  保存账号
                </n-button>
              </n-space>
            </n-form-item>
          </template>
        </n-form>
      </div>
    </n-card>

    <!-- OAuth2 配置弹窗 -->
    <n-modal v-model:show="showOAuth2ConfigModal" preset="dialog" title="配置 OAuth2">
      <n-form label-placement="left" label-width="120">
        <n-alert type="info" style="margin-bottom: 16px;">
          <template v-if="selectedVendor?.vendor === 'gmail'">
            请在 <a href="https://console.cloud.google.com/apis/credentials" target="_blank">Google Cloud Console</a> 创建 OAuth 2.0 客户端（选择"桌面应用"类型）
          </template>
          <template v-else>
            请在 <a href="https://portal.azure.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade" target="_blank">Azure Portal</a> 创建应用：<br/>
            1. 选择"任何组织目录中的帐户和个人 Microsoft 帐户"<br/>
            2. 重定向 URI 类型选择"移动和桌面应用程序"<br/>
            3. 添加 URI: <code>http://localhost</code>
          </template>
        </n-alert>
        <n-form-item label="Client ID">
          <n-input v-model:value="oauth2Config.clientId" placeholder="请输入 Client ID（应用程序 ID）" />
        </n-form-item>
        <n-form-item v-if="selectedVendor?.vendor === 'gmail'" label="Client Secret">
          <n-input v-model:value="oauth2Config.clientSecret" type="password" show-password-on="click" placeholder="请输入 Client Secret" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space>
          <n-button @click="showOAuth2ConfigModal = false">取消</n-button>
          <n-button type="primary" @click="handleSaveOAuth2Config">保存</n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- OAuth2 等待遮罩 -->
    <n-modal v-model:show="oauth2Waiting" :mask-closable="false" :close-on-esc="false">
      <n-card style="width: 400px; text-align: center;">
        <n-spin size="large" />
        <p style="margin-top: 16px;">请在浏览器中完成授权...</p>
        <n-button @click="handleCancelOAuth2">取消授权</n-button>
      </n-card>
    </n-modal>
  </div>
</template>

<style scoped>
.add-account-page {
  min-height: 100vh;
  background: #f5f5f5;
  padding: 24px;
}

.header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
  -webkit-app-region: drag;
  padding: 8px 0;
}

.header :deep(button) {
  -webkit-app-region: no-drag;
}

.header h2 {
  margin: 0;
  cursor: default;
}

.form-card {
  max-width: 800px;
  margin: 0 auto;
}

.section {
  margin-bottom: 24px;
}

.section h3 {
  margin: 0 0 16px;
  font-size: 16px;
  color: #333;
}

.vendor-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 16px;
  border: 2px solid #eee;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.vendor-card:hover {
  border-color: #18a058;
}

.vendor-card.active {
  border-color: #18a058;
  background: #f0faf4;
}

.vendor-icon {
  width: 40px;
  height: 40px;
  margin-bottom: 8px;
  object-fit: contain;
}

.vendor-name {
  font-size: 14px;
  color: #333;
}
</style>
