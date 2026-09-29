import { create } from 'zustand'

export type ToastVariant = 'success' | 'error' | 'info'

export interface Toast {
  id: number
  title: string
  description?: string
  variant: ToastVariant
}

interface UIState {
  paletteOpen: boolean
  setPaletteOpen: (open: boolean) => void
  /** 后台管理弹窗开关（首页齿轮、/admin 深度链接共用）。 */
  adminDialogOpen: boolean
  setAdminDialogOpen: (open: boolean) => void
  toasts: Toast[]
  pushToast: (toast: { title: string; description?: string; variant?: ToastVariant }) => void
  dismissToast: (id: number) => void
}

let toastSeq = 1

/** 轻量全局 UI 状态：搜索面板开关 + toast 队列。 */
export const useUI = create<UIState>((set) => ({
  paletteOpen: false,
  setPaletteOpen: (open) => set({ paletteOpen: open }),
  adminDialogOpen: false,
  setAdminDialogOpen: (open) => set({ adminDialogOpen: open }),
  toasts: [],
  pushToast: ({ title, description, variant = 'info' }) => {
    const id = toastSeq++
    set((state) => ({ toasts: [...state.toasts, { id, title, description, variant }] }))
    setTimeout(() => {
      set((state) => ({ toasts: state.toasts.filter((toast) => toast.id !== id) }))
    }, variant === 'error' ? 6000 : 3200)
  },
  dismissToast: (id) => set((state) => ({ toasts: state.toasts.filter((toast) => toast.id !== id) })),
}))

/** 统一的提示方法。 */
export const toast = {
  success: (title: string, description?: string) =>
    useUI.getState().pushToast({ title, description, variant: 'success' }),
  error: (title: string, description?: string) =>
    useUI.getState().pushToast({ title, description, variant: 'error' }),
  info: (title: string, description?: string) =>
    useUI.getState().pushToast({ title, description, variant: 'info' }),
}
