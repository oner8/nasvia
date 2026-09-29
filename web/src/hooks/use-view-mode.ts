import { useCallback, useEffect, useState } from 'react'

import {
  nextViewMode,
  normalizeViewMode,
  readStoredViewMode,
  writeStoredViewMode,
  type ViewMode,
} from '../lib/view-mode'

/** 安全取 localStorage（无 window 时返回 undefined，逻辑里会回退默认值）。 */
function currentStorage(): Storage | undefined {
  return typeof window === 'undefined' ? undefined : window.localStorage
}

export interface ViewModeState {
  view: ViewMode
  setView: (mode: ViewMode) => void
  toggle: () => void
}

/**
 * 视图模式（磁贴 / 应用）：初始值直接读 localStorage（纯 CSR，首帧即为用户选择，不会闪一下），
 * 变更后写回；写入失败只影响「下次记住」，不影响当前页面。
 */
export function useViewMode(): ViewModeState {
  const [view, setViewState] = useState<ViewMode>(() => readStoredViewMode(currentStorage()))

  useEffect(() => {
    writeStoredViewMode(currentStorage(), view)
  }, [view])

  const setView = useCallback((mode: ViewMode) => setViewState(normalizeViewMode(mode)), [])
  const toggle = useCallback(() => setViewState((prev) => nextViewMode(prev)), [])

  return { view, setView, toggle }
}
