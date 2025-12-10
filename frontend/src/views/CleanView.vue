<script lang="ts" setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useMessage } from 'naive-ui'
import {
  NLayout, NLayoutSider, NLayoutContent, NCard, NButton, NSpace, NTree, NDatePicker,
  NCheckbox, NProgress, NIcon, NTag, NSpin, NAlert, NScrollbar, NInputNumber, NInput,
  NSelect, NCollapse, NCollapseItem, NModal, NResult, NSkeleton
} from 'naive-ui'
import { ArrowBack, Trash, RefreshOutline } from '@vicons/ionicons5'
import { StartClean, CancelClean, GetAccount } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import { useAccountStore } from '../stores/account'
import { useFolderStore } from '../stores/folder'

interface FolderTreeNode {
  key: string
  label: string
  fullPath: string
  messageCount: number
  isLeaf: boolean
  disabled: boolean
  children?: FolderTreeNode[]
}

interface CleanProgress {
  currentFolder: string
  folderIndex: number
  totalFolders: number
  currentBatch: number
  totalBatches: number
  deletedCount: number
  matchedCount: number
  status: string
  message: string
  elapsedSeconds: number
}

const props = defineProps<{ accountId: string }>()
const router = useRouter()
const message = useMessage()
const accountStore = useAccountStore()
const folderStore = useFolderStore()

const loading = ref(false)
const cleaning = ref(false)
const folderTree = ref<FolderTreeNode[]>([])
const checkedKeys = ref<string[]>([])
const previewOnly = ref(true)
const startDate = ref<number | null>(null)
const endDate = ref<number | null>(null)
const batchSize = ref(500)
const maxConcurrency = ref(5)
// 高级筛选条件
const filterSender = ref('')
const filterSubject = ref('')
const filterSize = ref<string | null>(null)
const filterRead = ref<string | null>(null)

// 大小筛选选项
const sizeOptions = [
  { label: '不限', value: '' },
  { label: '大于 1MB', value: '>1M' },
  { label: '大于 5MB', value: '>5M' },
  { label: '大于 10MB', value: '>10M' },
  { label: '小于 100KB', value: '<100K' },
  { label: '小于 10KB', value: '<10K' }
]

// 已读/未读选项
const readOptions = [
  { label: '不限', value: '' },
  { label: '已读', value: 'seen' },
  { label: '未读', value: 'unseen' }
]

const progress = ref<CleanProgress | null>(null)
const progressLogs = ref<{ time: string; message: string }[]>([])
const account = ref<any>(null)
const expandedKeys = ref<string[]>([])
const cleanResult = ref<any>(null)
const showConfirmModal = ref(false)
const lastError = ref<string | null>(null)
const loadError = ref<string | null>(null)

// 快捷日期选项
const dateShortcuts = {
  '一年前': () => {
    const date = new Date()
    date.setFullYear(date.getFullYear() - 1)
    return date.getTime()
  },
  '半年前': () => {
    const date = new Date()
    date.setMonth(date.getMonth() - 6)
    return date.getTime()
  },
  '三个月前': () => {
    const date = new Date()
    date.setMonth(date.getMonth() - 3)
    return date.getTime()
  },
  '一个月前': () => {
    const date = new Date()
    date.setMonth(date.getMonth() - 1)
    return date.getTime()
  }
}

const progressPercent = computed(() => {
  if (!progress.value || progress.value.totalFolders === 0) return 0
  return Math.round((progress.value.folderIndex / progress.value.totalFolders) * 100)
})

// 获取所有文件夹的 key（递归）
const getAllFolderKeys = (nodes: FolderTreeNode[]): string[] => {
  const keys: string[] = []
  const traverse = (items: FolderTreeNode[]) => {
    for (const item of items) {
      keys.push(item.key)
      if (item.children && item.children.length > 0) {
        traverse(item.children)
      }
    }
  }
  traverse(nodes)
  return keys
}

// 是否全选
const isAllSelected = computed(() => {
  if (folderTree.value.length === 0) return false
  const allKeys = getAllFolderKeys(folderTree.value)
  return allKeys.length > 0 && allKeys.every(key => checkedKeys.value.includes(key))
})

// 全选/取消全选
const handleSelectAll = () => {
  if (isAllSelected.value) {
    checkedKeys.value = []
  } else {
    checkedKeys.value = getAllFolderKeys(folderTree.value)
  }
}

// 展开/折叠全部
const handleExpandAll = () => {
  if (expandedKeys.value.length > 0) {
    expandedKeys.value = []
  } else {
    expandedKeys.value = getAllFolderKeys(folderTree.value)
  }
}

// 格式化时间戳
const formatTimestamp = () => {
  const now = new Date()
  return now.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const handleBack = () => {
  router.push('/')
}

const loadFolders = async (forceRefresh = false) => {
  loading.value = true
  loadError.value = null
  try {
    const accountId = parseInt(props.accountId)
    account.value = await GetAccount(accountId)
    // 使用缓存的文件夹树
    folderTree.value = await folderStore.getFolderTree(accountId, forceRefresh)
    // 显示缓存状态
    const cacheInfo = folderStore.getCacheInfo(accountId)
    if (cacheInfo.cached && !forceRefresh && cacheInfo.age > 0) {
      message.info(`使用缓存数据 (${cacheInfo.age}秒前)`, { duration: 2000 })
    }
  } catch (error: any) {
    loadError.value = formatError(error)
    message.error(`加载文件夹失败: ${loadError.value}`)
  } finally {
    loading.value = false
  }
}

// 强制刷新文件夹
const refreshFolders = () => {
  loadFolders(true)
}

// 格式化错误信息
const formatError = (error: any): string => {
  const errorStr = String(error)
  if (errorStr.includes('connection refused') || errorStr.includes('network')) {
    return '网络连接失败，请检查网络设置或代理配置'
  }
  if (errorStr.includes('timeout')) {
    return '连接超时，请检查网络或稍后重试'
  }
  if (errorStr.includes('authentication') || errorStr.includes('auth')) {
    return '认证失败，请检查账号密码或重新授权'
  }
  if (errorStr.includes('IMAP')) {
    return 'IMAP 服务器连接失败，请检查服务器配置'
  }
  return errorStr
}

const formatDate = (timestamp: number) => {
  const date = new Date(timestamp)
  return date.toISOString().split('T')[0]
}

const handleStartClean = () => {
  if (checkedKeys.value.length === 0) {
    message.warning('请选择要清理的文件夹')
    return
  }
  if (!endDate.value) {
    message.warning('请选择结束时间')
    return
  }

  // 如果不是预览模式，显示确认对话框
  if (!previewOnly.value) {
    showConfirmModal.value = true
    return
  }

  doStartClean()
}

const doStartClean = async () => {
  showConfirmModal.value = false
  cleaning.value = true
  progress.value = null
  progressLogs.value = []
  cleanResult.value = null

  try {
    await StartClean({
      accountId: parseInt(props.accountId),
      folders: checkedKeys.value,
      startDate: startDate.value ? formatDate(startDate.value) : '',
      endDate: formatDate(endDate.value!),
      previewOnly: previewOnly.value,
      batchSize: batchSize.value,
      maxConcurrency: maxConcurrency.value,
      filterSender: filterSender.value,
      filterSubject: filterSubject.value,
      filterSize: filterSize.value || '',
      filterRead: filterRead.value || ''
    })
  } catch (error: any) {
    message.error(`启动清理失败: ${error}`)
    cleaning.value = false
  }
}

const handleCancelClean = () => {
  CancelClean()
}

const onProgress = (data: CleanProgress) => {
  progress.value = data
  if (data.message) {
    progressLogs.value.push({
      time: formatTimestamp(),
      message: data.message
    })
  }
}

const onComplete = (result: any) => {
  cleaning.value = false
  cleanResult.value = result
  message.success(`清理完成！共删除 ${result.totalDeleted} 封邮件`)
}

const onError = (error: string) => {
  cleaning.value = false
  lastError.value = formatError(error)
  message.error(`清理失败: ${lastError.value}`)
}

onMounted(() => {
  loadFolders()
  EventsOn('clean:progress', onProgress)
  EventsOn('clean:complete', onComplete)
  EventsOn('clean:error', onError)
})

onUnmounted(() => {
  EventsOff('clean:progress')
  EventsOff('clean:complete')
  EventsOff('clean:error')
})
</script>

<template>
  <n-layout class="clean-page" has-sider>
    <!-- 左侧文件夹选择 -->
    <n-layout-sider bordered :width="320" content-style="padding: 16px;" class="sider">
      <div class="sider-header">
        <n-button text @click="handleBack">
          <template #icon><n-icon><ArrowBack /></n-icon></template>
          返回
        </n-button>
        <n-button text @click="refreshFolders" :loading="loading" title="刷新文件夹列表">
          <template #icon><n-icon><RefreshOutline /></n-icon></template>
        </n-button>
      </div>

      <div v-if="account" class="account-info">
        <strong>{{ account.email }}</strong>
      </div>

      <!-- 加载错误提示 -->
      <n-alert v-if="loadError" type="error" style="margin-bottom: 12px;" closable @close="loadError = null">
        <template #header>加载失败</template>
        {{ loadError }}
        <n-button size="small" type="primary" style="margin-left: 12px;" @click="() => loadFolders()">
          重试
        </n-button>
      </n-alert>

      <div class="folder-header">
        <h3>📁 选择文件夹</h3>
        <n-space :size="4">
          <n-button size="tiny" quaternary @click="handleExpandAll">
            {{ expandedKeys.length > 0 ? '折叠' : '展开' }}
          </n-button>
          <n-button size="tiny" quaternary @click="handleSelectAll">
            {{ isAllSelected ? '取消全选' : '全选' }}
          </n-button>
        </n-space>
      </div>
      <!-- 骨架屏 -->
      <div v-if="loading" class="folder-skeleton">
        <n-skeleton v-for="i in 8" :key="i" :height="28" :width="i % 3 === 0 ? '60%' : i % 2 === 0 ? '80%' : '70%'" style="margin-bottom: 8px;" />
      </div>
      <!-- 文件夹树 -->
      <n-scrollbar v-else style="max-height: calc(100vh - 220px);">
        <n-tree
          :data="folderTree"
          checkable
          cascade
          selectable
          :checked-keys="checkedKeys"
          :expanded-keys="expandedKeys"
          @update:checked-keys="(keys: string[]) => checkedKeys = keys"
          @update:expanded-keys="(keys: string[]) => expandedKeys = keys"
          key-field="key"
          label-field="label"
          children-field="children"
          :render-suffix="({ option }: any) => option.messageCount > 0 ? ` (${option.messageCount})` : ''"
        />
      </n-scrollbar>
    </n-layout-sider>

    <!-- 右侧操作区 -->
    <n-layout-content content-style="padding: 24px;" class="content">
      <n-card title="筛选条件" size="small" style="margin-bottom: 16px;">
        <n-space vertical :size="12">
          <div class="filter-row">
            <label class="filter-label">开始时间：</label>
            <n-date-picker
              v-model:value="startDate"
              type="date"
              clearable
              :shortcuts="dateShortcuts"
              placeholder="可选，不填则不限制"
              style="width: 180px;"
            />
            <label class="filter-label" style="margin-left: 24px;">结束时间：</label>
            <n-date-picker
              v-model:value="endDate"
              type="date"
              clearable
              :shortcuts="dateShortcuts"
              placeholder="必填"
              style="width: 180px;"
            />
          </div>
          <!-- 高级筛选条件 -->
          <n-collapse>
            <n-collapse-item title="高级筛选" name="advanced">
              <n-space vertical :size="12">
                <div class="filter-row">
                  <label class="filter-label">发件人：</label>
                  <n-input
                    v-model:value="filterSender"
                    placeholder="jenny.ji@yunlsp.com"
                    :disabled="cleaning"
                    style="width: 350px;"
                  />
                </div>
                <div class="filter-row">
                  <label class="filter-label">主题包含：</label>
                  <n-input
                    v-model:value="filterSubject"
                    placeholder="主题关键词"
                    :disabled="cleaning"
                    style="width: 350px;"
                  />
                </div>
                <div class="filter-row">
                  <label class="filter-label">邮件大小：</label>
                  <n-select
                    v-model:value="filterSize"
                    :options="sizeOptions"
                    :disabled="cleaning"
                    placeholder="请选择"
                    style="width: 150px;"
                  />
                  <label class="filter-label" style="margin-left: 24px;">已读状态：</label>
                  <n-select
                    v-model:value="filterRead"
                    :options="readOptions"
                    :disabled="cleaning"
                    placeholder="请选择"
                    style="width: 120px;"
                  />
                </div>
              </n-space>
            </n-collapse-item>
          </n-collapse>

          <div class="filter-row">
            <label class="filter-label">批处理大小：</label>
            <n-input-number
              v-model:value="batchSize"
              :min="100"
              :max="2000"
              :step="100"
              :disabled="cleaning"
              style="width: 150px;"
            />
            <label class="filter-label" style="margin-left: 24px;">并发数：</label>
            <n-input-number
              v-model:value="maxConcurrency"
              :min="1"
              :max="10"
              :disabled="cleaning"
              style="width: 120px;"
            />
          </div>
          <n-checkbox v-model:checked="previewOnly">
            仅预览（不实际删除）
          </n-checkbox>
        </n-space>
      </n-card>

      <n-card title="操作" size="small" style="margin-bottom: 16px;">
        <n-space>
          <n-button
            type="primary"
            :loading="cleaning"
            :disabled="checkedKeys.length === 0 || !endDate"
            @click="handleStartClean"
          >
            <template #icon><n-icon><Trash /></n-icon></template>
            {{ previewOnly ? '预览清理' : '开始清理' }}
          </n-button>
          <n-button v-if="cleaning" @click="handleCancelClean">
            取消
          </n-button>
        </n-space>
        <div v-if="checkedKeys.length > 0" style="margin-top: 8px; color: #666;">
          已选择 {{ checkedKeys.length }} 个文件夹
        </div>
      </n-card>

      <!-- 清理错误提示 -->
      <n-alert v-if="lastError && !cleaning" type="error" style="margin-bottom: 16px;" closable @close="lastError = null">
        <template #header>清理失败</template>
        {{ lastError }}
        <n-button size="small" type="primary" style="margin-left: 12px;" @click="doStartClean">
          重试
        </n-button>
      </n-alert>

      <!-- 清理完成统计摘要 -->
      <n-card v-if="cleanResult && !cleaning" title="清理完成" size="small" style="margin-bottom: 16px;">
        <n-space vertical>
          <n-space>
            <n-tag type="success" size="large">
              {{ cleanResult.status === 'completed' ? '✓ 完成' : cleanResult.status === 'cancelled' ? '⚠ 已取消' : '✗ 失败' }}
            </n-tag>
          </n-space>
          <n-space :size="24">
            <div><strong>删除邮件：</strong>{{ cleanResult.totalDeleted }} 封</div>
            <div><strong>处理文件夹：</strong>{{ cleanResult.folderStats?.length || 0 }} 个</div>
            <div><strong>耗时：</strong>{{ cleanResult.duration?.toFixed(1) || 0 }}s</div>
          </n-space>
          <n-button size="small" @click="cleanResult = null">关闭</n-button>
        </n-space>
      </n-card>

      <!-- 进度显示 -->
      <n-card v-if="progress || progressLogs.length > 0" title="清理进度" size="small">
        <n-progress
          v-if="progress"
          type="line"
          :percentage="progressPercent"
          :status="progress.status === 'completed' ? 'success' : 'default'"
        />
        <div v-if="progress" style="margin: 8px 0;">
          <n-tag type="info">已删除: {{ progress.deletedCount }} 封</n-tag>
          <span style="margin-left: 8px; color: #666;">
            耗时: {{ progress.elapsedSeconds.toFixed(1) }}s
          </span>
        </div>
        <n-scrollbar style="max-height: 300px; margin-top: 12px;">
          <div class="progress-logs">
            <div v-for="(log, index) in progressLogs" :key="index" class="log-item">
              <span class="log-time">{{ log.time }}</span>
              <span class="log-message">{{ log.message }}</span>
            </div>
          </div>
        </n-scrollbar>
      </n-card>
    </n-layout-content>

    <!-- 确认删除对话框 -->
    <n-modal v-model:show="showConfirmModal" preset="dialog" title="确认清理">
      <template #icon>
        <n-icon color="#f0a020"><Trash /></n-icon>
      </template>
      <div style="padding: 16px 0;">
        <p><strong>⚠️ 警告：此操作将永久删除邮件！</strong></p>
        <p style="margin-top: 8px;">
          即将删除 <strong>{{ checkedKeys.length }}</strong> 个文件夹中符合条件的邮件。
        </p>
        <p style="margin-top: 8px; color: #666;">
          删除后无法恢复，请确认是否继续？
        </p>
      </div>
      <template #action>
        <n-space>
          <n-button @click="showConfirmModal = false">取消</n-button>
          <n-button type="error" @click="doStartClean">确认删除</n-button>
        </n-space>
      </template>
    </n-modal>
  </n-layout>
</template>

<style scoped>
.clean-page {
  height: 100vh;
}

.sider {
  background: #fafafa;
}

.sider-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  -webkit-app-region: drag;
  padding: 4px 0;
}

.sider-header :deep(button) {
  -webkit-app-region: no-drag;
}

.account-info {
  padding: 8px 12px;
  background: #e8f5e9;
  border-radius: 4px;
  margin-bottom: 12px;
}

.folder-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.folder-header h3 {
  margin: 0;
}

.content {
  background: #fff;
}

.progress-logs {
  font-family: monospace;
  font-size: 12px;
}

.log-item {
  padding: 4px 0;
  border-bottom: 1px solid #f0f0f0;
  display: flex;
  gap: 8px;
}

.log-item:last-child {
  border-bottom: none;
}

.log-time {
  color: #999;
  flex-shrink: 0;
}

.log-message {
  color: #333;
}

.folder-skeleton {
  padding: 8px 0;
}

.filter-row {
  display: flex;
  align-items: center;
}

.filter-label {
  width: 90px;
  flex-shrink: 0;
  text-align: right;
  padding-right: 8px;
  white-space: nowrap;
}
</style>
