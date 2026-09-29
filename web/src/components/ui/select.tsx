import { useMemo } from 'react'

import { Select as BaseSelect } from '@base-ui/react/select'
import { Switch as BaseSwitch } from '@base-ui/react/switch'
import { Check, ChevronDown } from 'lucide-react'

import { cn } from '../../lib/utils'

export interface SelectOption {
  value: string
  label: string
  description?: string
  /** 单项禁用（例如私密分类下不允许把站点设为公开）。 */
  disabled?: boolean
}

export interface SelectProps {
  value: string
  onChange: (value: string) => void
  options: SelectOption[]
  placeholder?: string
  className?: string
  disabled?: boolean
  ariaLabel?: string
}

/** 基于 @base-ui/react 的下拉选择。 */
export function Select({
  value,
  onChange,
  options,
  placeholder = '请选择',
  className,
  disabled,
  ariaLabel,
}: SelectProps) {
  /** value → 标签映射：触发器显示中文标签而不是原始值（public / private / inherit …）。 */
  const items = useMemo(
    () => options.map((option) => ({ value: option.value, label: option.label })),
    [options],
  )
  const labelOf = (current: unknown) =>
    options.find((option) => option.value === String(current ?? ''))?.label ?? placeholder

  return (
    <BaseSelect.Root
      value={value}
      onValueChange={(next) => onChange(String(next ?? ''))}
      disabled={disabled}
      items={items}
    >
      <BaseSelect.Trigger
        aria-label={ariaLabel}
        className={cn(
          'flex h-10 w-full items-center justify-between gap-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-card)] px-3 text-sm outline-none transition',
          'hover:bg-[var(--color-muted)] focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]/40 disabled:opacity-60',
          className,
        )}
      >
        <BaseSelect.Value placeholder={placeholder}>{(current) => labelOf(current)}</BaseSelect.Value>
        <BaseSelect.Icon className="text-[var(--color-muted-foreground)]">
          <ChevronDown className="size-4" />
        </BaseSelect.Icon>
      </BaseSelect.Trigger>
      <BaseSelect.Portal>
        <BaseSelect.Positioner sideOffset={6} alignItemWithTrigger={false} className="z-[60] outline-none">
          <BaseSelect.Popup
            className={cn(
              'max-h-72 min-w-[var(--anchor-width)] overflow-y-auto rounded-xl border border-[var(--color-border)] bg-[var(--color-card)] p-1 shadow-soft',
              'transition data-[starting-style]:scale-95 data-[starting-style]:opacity-0 data-[ending-style]:scale-95 data-[ending-style]:opacity-0',
            )}
          >
            <BaseSelect.List>
              {options.map((option) => (
                <BaseSelect.Item
                  key={option.value}
                  value={option.value}
                  disabled={option.disabled}
                  className="flex cursor-pointer items-center justify-between gap-3 rounded-lg px-2.5 py-2 text-sm outline-none data-[highlighted]:bg-[var(--color-muted)] data-[disabled]:cursor-not-allowed data-[disabled]:opacity-45"
                >
                  <span className="flex flex-col">
                    <BaseSelect.ItemText>{option.label}</BaseSelect.ItemText>
                    {option.description ? (
                      <span className="text-xs text-[var(--color-muted-foreground)]">{option.description}</span>
                    ) : null}
                  </span>
                  <BaseSelect.ItemIndicator>
                    <Check className="size-4 text-[var(--color-primary)]" />
                  </BaseSelect.ItemIndicator>
                </BaseSelect.Item>
              ))}
            </BaseSelect.List>
          </BaseSelect.Popup>
        </BaseSelect.Positioner>
      </BaseSelect.Portal>
    </BaseSelect.Root>
  )
}

export interface SwitchProps {
  checked: boolean
  onChange: (checked: boolean) => void
  disabled?: boolean
  ariaLabel?: string
  className?: string
}

/** 基于 @base-ui/react 的开关。 */
export function Switch({ checked, onChange, disabled, ariaLabel, className }: SwitchProps) {
  return (
    <BaseSwitch.Root
      checked={checked}
      onCheckedChange={(next) => onChange(next)}
      disabled={disabled}
      aria-label={ariaLabel}
      className={cn(
        'relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border border-transparent transition-colors outline-none',
        'focus-visible:ring-2 focus-visible:ring-[var(--color-ring)]/50 disabled:cursor-not-allowed disabled:opacity-50',
        checked ? 'bg-[var(--color-primary)]' : 'bg-[var(--color-muted)]',
        className,
      )}
    >
      <BaseSwitch.Thumb
        className={cn(
          'block size-4 rounded-full bg-white shadow transition-transform',
          checked ? 'translate-x-[1.1rem]' : 'translate-x-[0.15rem]',
        )}
      />
    </BaseSwitch.Root>
  )
}
