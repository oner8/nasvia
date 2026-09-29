import { useEffect, useRef } from 'react'
import { useNavigate } from '@tanstack/react-router'

import { useUI } from '../stores/ui'
import { HomePage } from './home-page'

/**
 * /admin 路由：渲染首页并自动打开后台弹窗（深度链接仍然可用），
 * 弹窗关闭后回到 `/`。
 */
export function AdminPage() {
  const open = useUI((state) => state.adminDialogOpen)
  const setAdminDialogOpen = useUI((state) => state.setAdminDialogOpen)
  const navigate = useNavigate()
  const wasOpen = useRef(false)

  useEffect(() => {
    setAdminDialogOpen(true)
    return () => setAdminDialogOpen(false)
  }, [setAdminDialogOpen])

  useEffect(() => {
    if (open) {
      wasOpen.current = true
      return
    }
    if (wasOpen.current) {
      wasOpen.current = false
      void navigate({ to: '/', replace: true })
    }
  }, [open, navigate])

  return <HomePage />
}
