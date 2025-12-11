import { defineStore } from 'pinia'
import { ref } from 'vue'
import { GetFolderTree } from '../../wailsjs/go/main/App'

interface FolderTreeNode {
  key: string
  label: string
  fullPath: string
  messageCount: number
  isLeaf: boolean
  disabled: boolean
  children?: FolderTreeNode[]
}

interface CacheEntry {
  data: FolderTreeNode[]
  timestamp: number
}

// 缓存有效期：5分钟
const CACHE_TTL = 5 * 60 * 1000

export const useFolderStore = defineStore('folder', () => {
  // 按账号ID缓存文件夹树
  const cache = ref<Map<number, CacheEntry>>(new Map())
  const loading = ref(false)

  // 检查缓存是否有效
  function isCacheValid(accountId: number): boolean {
    const entry = cache.value.get(accountId)
    if (!entry) return false
    return Date.now() - entry.timestamp < CACHE_TTL
  }

  // 获取文件夹树（优先使用缓存）
  async function getFolderTree(accountId: number, forceRefresh = false): Promise<FolderTreeNode[]> {
    // 如果不强制刷新且缓存有效，直接返回缓存
    if (!forceRefresh && isCacheValid(accountId)) {
      return cache.value.get(accountId)!.data
    }

    loading.value = true
    try {
      const data = await GetFolderTree(accountId)
      // 更新缓存
      cache.value.set(accountId, {
        data: data || [],
        timestamp: Date.now()
      })
      return data || []
    } finally {
      loading.value = false
    }
  }

  // 清除指定账号的缓存
  function clearCache(accountId: number) {
    cache.value.delete(accountId)
  }

  // 清除所有缓存
  function clearAllCache() {
    cache.value.clear()
  }

  // 获取缓存状态
  function getCacheInfo(accountId: number): { cached: boolean; age: number } {
    const entry = cache.value.get(accountId)
    if (!entry) {
      return { cached: false, age: 0 }
    }
    return {
      cached: true,
      age: Math.round((Date.now() - entry.timestamp) / 1000)
    }
  }

  // 更新缓存中指定文件夹的邮件数量
  function updateFolderStatus(accountId: number, folderPath: string, messageCount: number) {
    const entry = cache.value.get(accountId)
    if (!entry) return

    const updateNode = (nodes: FolderTreeNode[]): boolean => {
      for (const node of nodes) {
        if (node.fullPath === folderPath) {
          node.messageCount = messageCount
          return true
        }
        if (node.children && updateNode(node.children)) {
          return true
        }
      }
      return false
    }
    updateNode(entry.data)
  }

  return {
    loading,
    getFolderTree,
    clearCache,
    clearAllCache,
    getCacheInfo,
    isCacheValid,
    updateFolderStatus
  }
})

