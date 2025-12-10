import { defineStore } from 'pinia'
import { ref } from 'vue'
import { ListAccounts, DeleteAccount } from '../../wailsjs/go/main/App'
import { model } from '../../wailsjs/go/models'

export const useAccountStore = defineStore('account', () => {
  const accounts = ref<model.AccountListItem[]>([])
  const loading = ref(false)
  const currentAccountId = ref<number | null>(null)

  async function fetchAccounts() {
    loading.value = true
    try {
      const list = await ListAccounts()
      accounts.value = list || []
    } catch (error) {
      console.error('获取账号列表失败:', error)
      accounts.value = []
    } finally {
      loading.value = false
    }
  }

  async function removeAccount(id: number) {
    try {
      await DeleteAccount(id)
      accounts.value = accounts.value.filter(a => a.id !== id)
      return true
    } catch (error) {
      console.error('删除账号失败:', error)
      return false
    }
  }

  function setCurrentAccount(id: number | null) {
    currentAccountId.value = id
  }

  function getCurrentAccount() {
    if (!currentAccountId.value) return null
    return accounts.value.find(a => a.id === currentAccountId.value) || null
  }

  return {
    accounts,
    loading,
    currentAccountId,
    fetchAccounts,
    removeAccount,
    setCurrentAccount,
    getCurrentAccount
  }
})

