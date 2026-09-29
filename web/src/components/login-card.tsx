import { useState } from 'react'
import type { FormEvent } from 'react'
import { KeyRound, Loader2, ShieldAlert } from 'lucide-react'

import { apiErrorMessage } from '../lib/api'
import { useLogin } from '../hooks/use-api'
import { toast } from '../stores/ui'
import { Button } from './ui/button'
import { Input, Label } from './ui/input'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/primitives'

export interface LoginFormProps {
  title?: string
  description?: string
  /** 未设置密码时的说明文案（此时禁止登录）。 */
  passwordMissing?: boolean
  /** 表单内 input id 前缀，避免同页多实例冲突。 */
  idPrefix?: string
  /** 是否显示表单自带标题区（卡片模式显示；弹窗模式由弹窗外壳提供标题）。 */
  showHeading?: boolean
}

/** 登录表单本体（无卡片外壳）：前台「需登录」模式与后台弹窗共用。 */
export function LoginForm({
  title = '需要登录',
  description,
  passwordMissing,
  idPrefix = 'nasvia',
  showHeading = true,
}: LoginFormProps) {
  const [password, setPassword] = useState('')
  const login = useLogin()
  const inputId = `${idPrefix}-password`

  const onSubmit = async (event: FormEvent) => {
    event.preventDefault()
    if (!password) {
      toast.error('请输入密码')
      return
    }
    try {
      await login.mutateAsync(password)
      setPassword('')
      toast.success('登录成功')
    } catch (error) {
      toast.error('登录失败', apiErrorMessage(error))
    }
  }

  return (
    <div className="flex flex-col gap-4">
      {showHeading ? (
        <div>
          <p className="flex items-center gap-2 text-sm font-semibold tracking-tight">
            <KeyRound className="size-4" />
            {title}
          </p>
          {description ? (
            <p className="mt-1 text-xs leading-relaxed text-[var(--color-muted-foreground)]">
              {description}
            </p>
          ) : null}
        </div>
      ) : null}

      {passwordMissing ? (
        <div className="flex items-start gap-2 rounded-xl border border-[var(--color-danger)]/35 bg-[var(--color-danger)]/8 p-3 text-xs leading-relaxed text-[var(--color-danger)]">
          <ShieldAlert className="mt-0.5 size-4 shrink-0" />
          <span>
            未设置密码，后台与登录都不可用。请设置环境变量 <code className="font-mono">NASVIA_PASSWORD</code>
            ，或在本机直接访问并在后台「设置」中设置密码。
          </span>
        </div>
      ) : (
        <form onSubmit={onSubmit} className="flex flex-col gap-3">
          <div>
            <Label htmlFor={inputId}>后台密码</Label>
            <Input
              id={inputId}
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder="请输入密码"
            />
          </div>
          <Button type="submit" disabled={login.isPending}>
            {login.isPending ? <Loader2 className="size-4 animate-spin" /> : null}
            登录
          </Button>
        </form>
      )}
    </div>
  )
}

export interface LoginCardProps extends LoginFormProps {}

/** 卡片形态的登录：用于前台「需登录」模式的整页拦截。 */
export function LoginCard({
  title = '需要登录',
  description,
  passwordMissing,
  idPrefix = 'nasvia',
}: LoginCardProps) {
  return (
    <Card className="mx-auto w-full max-w-sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <KeyRound className="size-4" />
          {title}
        </CardTitle>
        <CardDescription>
          {description ?? '该实例已设为「私密」模式，登录后即可查看全部服务。'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <LoginForm
          passwordMissing={passwordMissing}
          idPrefix={idPrefix}
          showHeading={false}
        />
      </CardContent>
    </Card>
  )
}
