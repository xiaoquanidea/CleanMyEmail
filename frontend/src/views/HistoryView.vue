<script lang="ts" setup>
import { ref, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import {
  NLayout, NLayoutContent, NCard, NButton, NSpace, NIcon, NPageHeader,
  NDataTable, NTag, NEmpty, NPopconfirm, NSpin
} from 'naive-ui'
import { ArrowBack, Trash, RefreshOutline } from '@vicons/ionicons5'
import { GetCleanHistoryList, DeleteCleanHistory, ClearAllCleanHistory } from '../../wailsjs/go/main/App'
import { model } from '../../wailsjs/go/models'

const router = useRouter()
const message = useMessage()

const loading = ref(false)
const historyList = ref<model.CleanHistoryListItem[]>([])

type HistoryRow = model.CleanHistoryListItem

const columns = [
  {
    title: '时间',
    key: 'createdAt',
    width: 160,
    render: (row: HistoryRow) => formatTime(row.createdAt)
  },
  { title: '账号', key: 'accountEmail', ellipsis: { tooltip: true } },
  { title: '文件夹', key: 'folderCount', width: 80 },
  { title: '日期范围', key: 'dateRange', width: 180 },
  { title: '匹配', key: 'matchedCount', width: 80 },
  { title: '删除', key: 'deletedCount', width: 80 },
  {
    title: '类型',
    key: 'previewOnly',
    width: 80,
    render: (row: HistoryRow) => row.previewOnly ? '预览' : '清理'
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    render: (row: HistoryRow) => {
      const statusMap: Record<string, { type: 'success' | 'error' | 'warning' | 'info', text: string }> = {
        completed: { type: 'success', text: '完成' },
        failed: { type: 'error', text: '失败' },
        cancelled: { type: 'warning', text: '取消' },
        running: { type: 'info', text: '进行中' }
      }
      const s = statusMap[row.status] || { type: 'info', text: row.status }
      return h(NTag, { type: s.type, size: 'small' }, { default: () => s.text })
    }
  },
  {
    title: '耗时',
    key: 'duration',
    width: 80,
    render: (row: HistoryRow) => row.duration > 0 ? `${row.duration.toFixed(1)}s` : '-'
  },
  {
    title: '操作',
    key: 'actions',
    width: 80,
    render: (row: HistoryRow) => {
      return h(NPopconfirm, {
        onPositiveClick: () => handleDelete(row.id)
      }, {
        trigger: () => h(NButton, { text: true, type: 'error', size: 'small' }, {
          icon: () => h(NIcon, null, { default: () => h(Trash) })
        }),
        default: () => '确定删除此记录？'
      })
    }
  }
]

const formatTime = (time: any) => {
  // 处理 Go 的 time.Time 类型（可能是字符串或对象）
  const dateStr = typeof time === 'string' ? time : (time?.Time || time)
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit'
  })
}

const loadHistory = async () => {
  loading.value = true
  try {
    historyList.value = await GetCleanHistoryList(100, 0) || []
  } catch (e: any) {
    message.error('加载历史记录失败: ' + e)
  } finally {
    loading.value = false
  }
}

const handleDelete = async (id: number) => {
  try {
    await DeleteCleanHistory(id)
    message.success('删除成功')
    loadHistory()
  } catch (e: any) {
    message.error('删除失败: ' + e)
  }
}

const handleClearAll = async () => {
  try {
    await ClearAllCleanHistory()
    message.success('已清空所有历史记录')
    historyList.value = []
  } catch (e: any) {
    message.error('清空失败: ' + e)
  }
}

const handleBack = () => {
  router.push('/')
}

onMounted(() => {
  loadHistory()
})
</script>

<template>
  <n-layout class="layout">
    <n-layout-content content-style="padding: 24px;">
      <n-page-header @back="handleBack" title="清理历史" subtitle="查看历史清理记录">
        <template #back>
          <n-icon><ArrowBack /></n-icon>
        </template>
        <template #extra>
          <n-space>
            <n-button @click="loadHistory" :loading="loading">
              <template #icon><n-icon><RefreshOutline /></n-icon></template>
              刷新
            </n-button>
            <n-popconfirm @positive-click="handleClearAll">
              <template #trigger>
                <n-button type="error" :disabled="historyList.length === 0">
                  清空全部
                </n-button>
              </template>
              确定清空所有历史记录？
            </n-popconfirm>
          </n-space>
        </template>
      </n-page-header>

      <n-card style="margin-top: 16px;">
        <n-spin :show="loading">
          <n-data-table
            v-if="historyList.length > 0"
            :columns="columns"
            :data="historyList"
            :bordered="false"
            size="small"
          />
          <n-empty v-else description="暂无清理历史记录" />
        </n-spin>
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

