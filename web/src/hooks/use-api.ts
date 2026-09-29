import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api } from '../lib/api'
import type { AppConfig, CategoryListResponse, SiteListResponse } from '../lib/types'

/** 公开配置（含登录态、内网网段、前台模式）。 */
export function useConfig() {
  return useQuery<AppConfig>({
    queryKey: ['config'],
    queryFn: () => api.config(),
    staleTime: 15_000,
  })
}

// 站点图标是服务端异步抓取的：只要有站点还处于 pending，就轮询列表让新图标自己出现。
// 30 秒后放弃（例如来源全被关掉时 pending 不会结束），避免一直轮询。
let iconPollSince = 0
export function iconPollInterval(query: { state: { data?: SiteListResponse } }): number | false {
	const pending = (query.state.data?.items ?? []).some((site) => site.icon_state === 'pending')
	if (!pending) {
		iconPollSince = 0
		return false
	}
	if (iconPollSince === 0) iconPollSince = Date.now()
	if (Date.now() - iconPollSince > 30_000) return false
	return 1500
}

/** 站点列表；private 模式未登录时后端返回 401，此时不重试。 */
export function useSites(enabled = true) {
  return useQuery<SiteListResponse>({
    queryKey: ['sites'],
    queryFn: () => api.sites(),
    enabled,
    refetchInterval: iconPollInterval,
    retry: false,
  })
}

export function useCategories(enabled = true) {
  return useQuery<CategoryListResponse>({
    queryKey: ['categories'],
    queryFn: () => api.categories(),
    enabled,
    retry: false,
  })
}

export function useAdminSites(enabled = true) {
  return useQuery<SiteListResponse>({
    queryKey: ['admin', 'sites'],
    queryFn: () => api.adminSites(),
    enabled,
    refetchInterval: iconPollInterval,
  })
}

export function useAdminCategories(enabled = true) {
  return useQuery<CategoryListResponse>({
    queryKey: ['admin', 'categories'],
    queryFn: () => api.adminCategories(),
    enabled,
  })
}

export function useAdminSettings(enabled = true) {
  return useQuery({
    queryKey: ['admin', 'settings'],
    queryFn: () => api.settings(),
    enabled,
  })
}

export function useLogin() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (password: string) => api.login(password),
    onSuccess: async () => {
      await queryClient.invalidateQueries()
    },
  })
}

export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => api.logout(),
    onSuccess: async () => {
      await queryClient.invalidateQueries()
    },
  })
}

/** 每秒刷新一次的本地时钟。 */
export function useClock(intervalMs = 1000) {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const timer = window.setInterval(() => setNow(new Date()), intervalMs)
    return () => window.clearInterval(timer)
  }, [intervalMs])
  return now
}

/** 随窗口尺寸变化重新计算的断点标记（用于响应式细节）。 */
export function useIsMobile(breakpoint = 768) {
  const [isMobile, setIsMobile] = useState(
    () => typeof window !== 'undefined' && window.innerWidth < breakpoint,
  )
  useEffect(() => {
    const onResize = () => setIsMobile(window.innerWidth < breakpoint)
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [breakpoint])
  return isMobile
}
