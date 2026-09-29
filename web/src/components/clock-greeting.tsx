import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import type { LucideIcon } from 'lucide-react'
import { Coffee, Hourglass, Moon, MoonStar, Sun, SunMedium, Sunrise, Sunset, Utensils } from 'lucide-react'
import { motion } from 'motion/react'
import type { ReactNode } from 'react'

import { useClock } from '../hooks/use-api'
import { clockLabel, clockParts } from '../lib/clock-digits'
import { greetingForDate, type GreetingPeriod } from '../lib/greeting'

dayjs.locale('zh-cn')

const DIGITS = ['0', '1', '2', '3', '4', '5', '6', '7', '8', '9'] as const

/** 问候时段 → 图标（与文案 emoji 的语义对齐）。 */
const GREETING_ICONS: Record<GreetingPeriod, LucideIcon> = {
  lateNight: MoonStar,
  dawn: Sunrise,
  earlyMorning: Sun,
  morning: SunMedium,
  noon: Utensils,
  afternoon: Coffee,
  wrapUp: Hourglass,
  dusk: Sunset,
  evening: Moon,
}

/** 数字格高度（em）：比字形更高，让上下淡出遮罩落在字形之外，避免裁切。 */
const CELL_EM = 1.5

/** 单个数字格：数字垂直居中，保证六位数字严格对齐。 */
function DigitCell({ children }: { children: ReactNode }) {
  return (
    <span className="flex h-[1.5em] w-[0.62em] items-center justify-center leading-none">
      {children}
    </span>
  )
}

/**
 * 转筒式数字：0-9 纵向排列，按当前值滚动，
 * 配合 roll-mask 做上下淡出，与相邻数字形成「滚筒」观感。
 */
function RollDigit({ value }: { value: string }) {
  return (
    <span className="roll-mask relative inline-block h-[1.5em] w-[0.62em] overflow-hidden align-middle">
      <motion.span
        className="absolute inset-x-0 top-0 flex flex-col items-center"
        initial={false}
        animate={{ y: `-${Number(value) * CELL_EM}em` }}
        transition={{ type: 'spring', stiffness: 260, damping: 30, mass: 0.55 }}
      >
        {DIGITS.map((digit) => (
          <DigitCell key={digit}>{digit}</DigitCell>
        ))}
      </motion.span>
    </span>
  )
}

/** 一对数字（例如「07」）。 */
function DigitPair({ value }: { value: string }) {
  return (
    <>
      <RollDigit value={value[0]} />
      <RollDigit value={value[1]} />
    </>
  )
}

/** 工具栏左侧：时段问候（带图标）+ 等宽实时时钟，秒位为转筒数字。 */
export function ClockGreeting() {
  const now = useClock(1000)
  const greeting = greetingForDate(now)
  const Icon = GREETING_ICONS[greeting.key] ?? Sun
  const { hours, minutes, seconds } = clockParts(now)

  return (
    <div className="text-xs text-[var(--color-muted-foreground)]">
      <div className="flex items-center gap-1.5" title={dayjs(now).format('YYYY年M月D日 dddd')}>
        <Icon className="size-3.5" aria-hidden />
        <span>{greeting.text}</span>
      </div>
      <time
        dateTime={now.toISOString()}
        aria-label={clockLabel(now)}
        className="mt-1 flex items-center text-[var(--color-foreground)]"
      >
        <DigitPair value={hours} />
        <span aria-hidden className="mx-px opacity-50">
          :
        </span>
        <DigitPair value={minutes} />
        <span aria-hidden className="mx-px opacity-50">
          :
        </span>
        <DigitPair value={seconds} />
      </time>
    </div>
  )
}
