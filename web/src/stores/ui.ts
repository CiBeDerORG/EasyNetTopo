import { defineStore } from 'pinia'
import { readThemeMode, setThemeMode, type ThemeMode } from './theme'

export type AppFilter = 'all' | 'installed' | 'available' | 'update'

export const useHostStoreUiStore = defineStore('host-store-ui', {
  state: () => ({
    sidebarCollapsed: false,
    selectedHostId: '',
    appFilter: 'all' as AppFilter,
    themeMode: readThemeMode() as ThemeMode
  }),
  actions: {
    selectHost(id: string) {
      this.selectedHostId = id
    },
    toggleSidebar() {
      this.sidebarCollapsed = !this.sidebarCollapsed
    },
    setThemeMode(mode: ThemeMode) {
      this.themeMode = mode
      setThemeMode(mode)
    }
  }
})
