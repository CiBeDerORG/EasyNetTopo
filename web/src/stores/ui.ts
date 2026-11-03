import { defineStore } from 'pinia'

export type AppFilter = 'all' | 'installed' | 'available' | 'update'

export const useHostStoreUiStore = defineStore('host-store-ui', {
  state: () => ({
    sidebarCollapsed: false,
    selectedHostId: '',
    appFilter: 'all' as AppFilter
  }),
  actions: {
    selectHost(id: string) {
      this.selectedHostId = id
    },
    toggleSidebar() {
      this.sidebarCollapsed = !this.sidebarCollapsed
    }
  }
})
