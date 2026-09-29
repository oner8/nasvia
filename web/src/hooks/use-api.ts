import { useCallback, useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { api } from '../lib/api'
import { ICON_POLL_MAX_MS, iconPollDelay, mergeIconStatuses } from '../lib/icon-poll'
import type { AppConfig, CategoryListResponse, SiteListResponse } from '../lib/types'

/** 公开配置（含登录态、内网网段、前台模式）。 */
export function useConfig() {
  return useQuery<AppConfig>({
    queryKey: ['config'],
    queryFn: () => api.config(),
    staleTime: 15_000,
  })
}

const ICON_STATUS_BATCH_SIZE = 100

function useIconStatusPolling(items: SiteListResponse['items'], admin: boolean, enabled: boolean) {
  const queryClient = useQueryClient()
  const startedAt = useRef(0)
  const failures = useRef(0)
  const retryNow = useRef(false)
  const [timedOut, setTimedOut] = useState(false)
  const [retryVersion, setRetryVersion] = useState(0)
  const pendingIDs = items.filter((site) => site.icon_state === 'pending').map((site) => site.id)
  const pendingKey = pendingIDs.join(',')

  useEffect(() => {
    if (!enabled || !pendingKey) {
      startedAt.current = 0
      failures.current = 0
      setTimedOut(false)
      return
    }
    if (startedAt.current === 0) startedAt.current = Date.now()

    let timer = 0
    let controller: AbortController | undefined
    let stopped = false

    const schedule = (delay?: number) => {
      if (stopped || document.hidden) return
      const elapsed = Date.now() - startedAt.current
      if (elapsed >= ICON_POLL_MAX_MS) {
        setTimedOut(true)
        return
      }
      timer = window.setTimeout(poll, delay ?? iconPollDelay(elapsed, failures.current))
    }

    const poll = async () => {
      controller = new AbortController()
      try {
        const results = await Promise.all(
          Array.from({ length: Math.ceil(pendingIDs.length / ICON_STATUS_BATCH_SIZE) }, (_, index) =>
            api.iconStatuses(
              pendingIDs.slice(index * ICON_STATUS_BATCH_SIZE, (index + 1) * ICON_STATUS_BATCH_SIZE),
              admin,
              controller?.signal,
            ),
          ),
        )
        failures.current = 0
        const queryKey = admin ? ['admin', 'sites'] : ['sites']
        queryClient.setQueryData<SiteListResponse>(queryKey, (current) =>
          mergeIconStatuses(current, results.flatMap((result) => result.items)),
        )
      } catch (error) {
        if (!controller.signal.aborted) failures.current += 1
      }
      schedule()
    }

    const onVisibility = () => {
      window.clearTimeout(timer)
      if (!document.hidden) schedule(0)
    }
    document.addEventListener('visibilitychange', onVisibility)
    const immediate = retryNow.current
    retryNow.current = false
    schedule(immediate ? 0 : undefined)
    return () => {
      stopped = true
      window.clearTimeout(timer)
      controller?.abort()
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [admin, enabled, pendingKey, queryClient, retryVersion])

  const retryIconPolling = useCallback(() => {
    startedAt.current = Date.now()
    failures.current = 0
    retryNow.current = true
    setTimedOut(false)
    setRetryVersion((value) => value + 1)
  }, [])

  return { iconPollingTimedOut: timedOut, retryIconPolling }
}

/** 站点列表；private 模式未登录时后端返回 401，此时不重试。 */
export function useSites(enabled = true) {
  const query = useQuery<SiteListResponse>({
    queryKey: ['sites'],
    queryFn: () => api.sites(),
    enabled,
    retry: false,
  })
  return { ...query, ...useIconStatusPolling(query.data?.items ?? [], false, enabled) }
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
  const query = useQuery<SiteListResponse>({
    queryKey: ['admin', 'sites'],
    queryFn: () => api.adminSites(),
    enabled,
  })
  return { ...query, ...useIconStatusPolling(query.data?.items ?? [], true, enabled) }
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
