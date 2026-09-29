import { useConfig } from '../../hooks/use-api'
import { BrandMark } from '../brand-logo'
import { LoginForm } from '../login-card'
import { Dialog } from '../ui/dialog'
import { AdminPanel } from './admin-panel'

export interface AdminDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

/**
 * 后台弹窗：未登录时是一个**窄弹窗**只放登录表单（表单自带标题与说明），
 * 登录后原地扩宽为管理面板（站点 / 分类 / 设置）。
 */
export function AdminDialog({ open, onOpenChange }: AdminDialogProps) {
  const configQuery = useConfig()
  const config = configQuery.data
  const showPanel = config?.authenticated ?? false
  const mode = config?.auth_mode === 'private' ? '私密' : '公开'
  const siteName = (config?.site_title ?? '').trim()
  const description = config
    ? [siteName && siteName !== 'NASVIA' ? siteName : null, `NASVIA v${config.version}`, mode]
        .filter(Boolean)
        .join(' · ')
    : undefined

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      className={showPanel ? 'w-[min(96vw,70rem)]' : 'w-[min(92vw,24rem)]'}
      title={
        showPanel ? (
          <span className="flex items-center gap-2">
            <BrandMark className="size-6" />
            后台管理
          </span>
        ) : undefined
      }
      description={showPanel ? description : undefined}
    >
      {configQuery.isLoading ? (
        <p className="py-8 text-center text-xs text-[var(--color-muted-foreground)]">正在加载…</p>
      ) : showPanel ? (
        <AdminPanel />
      ) : (
        <LoginForm
          idPrefix="admin"
          title="后台登录"
          description="后台在任何访问模式下都需要密码登录。"
          passwordMissing={config?.password_configured === false}
        />
      )}
    </Dialog>
  )
}
