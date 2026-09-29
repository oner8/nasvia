/**
 * 智能问候语：按时段（9 段，精确到分钟）+ 周末专属文案 + 节日彩蛋，
 * 并在固定时间窗内轮换措辞（同一时间窗内稳定，避免每次渲染都变）。
 *
 * 时段按常见上班节奏划分：上午上班 9:00–12:00、午休 12:00–14:00、
 * 下午上班 14:00–16:30、收尾（快下班）16:30–17:30、下班后 17:30 起。
 * 因此 13:36 属于「午休」，不会出现「快下班了」这类收尾话术。
 *
 * 周末与工作日用**两套完全独立的文案**：周六日只从 WEEKEND_GREETINGS 里选，
 * 工作日只从 GREETINGS 里选——避免工作日措辞漏到周末（早期周末文案只写了
 * 三个时段，其余时段会退回工作日池，周日下午就冒出了「再坚持一下，快下班了」）。
 *
 * 农历节日（春节 / 除夕 / 元宵 / 端午 / 七夕 / 中秋 / 重阳 / 腊八）用内置中文日历
 * （Intl chinese calendar）判定，不硬编码日期表；运行环境不支持该日历时自动跳过
 * 农历彩蛋。全部离线推算，不依赖任何外部接口。
 */

export type GreetingPeriod =
  | 'lateNight'
  | 'dawn'
  | 'earlyMorning'
  | 'morning'
  | 'noon'
  | 'afternoon'
  | 'wrapUp'
  | 'dusk'
  | 'evening'

export type FestivalId =
  | 'newYear'
  | 'dragonBoat'
  | 'midAutumn'
  | 'nationalDay'
  | 'christmas'
  | 'newYearEve'
  | 'lantern'
  | 'qixi'
  | 'doubleNinth'
  | 'laba'

export interface Greeting {
  /** 时段标识（用于图标）。 */
  key: GreetingPeriod
  /** 展示文案。 */
  text: string
  /** 命中的节日彩蛋。 */
  festival?: FestivalId
  /** 今天是否周末。 */
  weekend: boolean
}

/** 时段起点（自 0 点起的分钟数）；小于首个起点视为「深夜」。 */
const PERIOD_BOUNDS: ReadonlyArray<{ key: GreetingPeriod; start: number }> = [
  { key: 'dawn', start: 5 * 60 },
  { key: 'earlyMorning', start: 7 * 60 },
  { key: 'morning', start: 9 * 60 },
  { key: 'noon', start: 12 * 60 },
  { key: 'afternoon', start: 14 * 60 },
  { key: 'wrapUp', start: 16 * 60 + 30 },
  { key: 'dusk', start: 17 * 60 + 30 },
  { key: 'evening', start: 19 * 60 },
  { key: 'lateNight', start: 22 * 60 },
]

/** 工作日各时段文案。 */
export const GREETINGS: Record<GreetingPeriod, readonly string[]> = {
  lateNight: [
    '夜深了，该睡了 🌙',
    '这个点还不睡，明天会后悔的 😴',
    '月亮都困了，你也该休息了 🌛',
    '熬夜一时爽，早上火葬场 🔥',
    '夜猫子，注意身体 🦉',
    '世界都睡了，你也该睡了 💤',
  ],
  dawn: ['早安，新的一天开始了 🌅', '清晨好，今天也要加油 🌄', '清晨的空气真好 🍃'],
  earlyMorning: ['早上好，准备出发 ☀️', '早安，今天也要加油 💪', '新的一天，从一杯咖啡开始 ☕'],
  morning: ['上午好，专注做事 💻', '上午好，效率最高的时段 ⚡', '上午好，把重要的事先做完 🎯'],
  noon: [
    '中午了，记得吃午饭 🍜',
    '干饭时间到 🍚',
    '午休一会儿，下午还有半天 😌',
    '快到两点开工了，收拾下状态 ☕',
  ],
  afternoon: [
    '下午好，继续加油 ☕',
    '下午好，来杯茶提提神 🍵',
    '下午好，按自己的节奏推进 📈',
  ],
  wrapUp: [
    '再坚持一下，快下班了 ⏳',
    '收个尾，把今天的事收一收 📝',
    '快下班了，别忘了同步进度 📋',
  ],
  dusk: ['下班了，收拾收拾 🌇', '今天辛苦了，回家路上注意安全 🌆', '傍晚了，出去走走 🚶'],
  evening: ['晚上好，放松一下 🌃', '晚饭后散散步，对身体好 🚶', '晚上好，享受夜晚 🌌'],
}

/**
 * 周末专属文案：**覆盖全部 9 个时段**，周六日只用这一套。
 * 措辞一律不含「下班 / 上班 / 加班」这类只在工作日成立的词。
 */
export const WEEKEND_GREETINGS: Record<GreetingPeriod, readonly string[]> = {
  lateNight: [
    '周末夜色，熬夜也理直气壮 🌙',
    '夜深了，明天可以睡到自然醒 😴',
    '周末的深夜，安静又自在 🌛',
  ],
  dawn: ['周末早起，元气满满 🌞', '清晨好，周末的清晨格外安静 🌄', '周末清晨，慢慢来 🍃'],
  earlyMorning: [
    '周末早安，今天不用赶时间 ☀️',
    '早安，周末从一顿好早餐开始 🍳',
    '周末的早上，先伸个懒腰 🙆',
  ],
  morning: ['上午好，享受周末 🛋️', '周末上午，做点喜欢的事 🎧', '周末上午，晒晒太阳正好 🌤️'],
  noon: ['周末午饭时间，吃点好的 🍜', '中午了，周末午餐慢慢享受 🍚', '周末中午，饭后小憩一会儿 😌'],
  afternoon: [
    '周末午后，惬意时光 🌤️',
    '午后好，泡杯茶放空一下 🍵',
    '周末下午，出门走走也不错 🚶',
  ],
  wrapUp: ['周末这会儿，适合小睡一觉 😴', '休息日不用赶时间，慢慢来 ☕', '下午茶时间，犒劳一下自己 🍰'],
  dusk: ['周末傍晚，散步正好 🌇', '傍晚好，周末的风很舒服 🌆', '周末的黄昏，适合发会儿呆 🌅'],
  evening: ['周末的夜晚，放松一下 🍷', '晚上好，周末愉快 🌃', '周末夜，看部电影吧 🎬'],
}

/** 节日彩蛋文案（当天整句优先）。 */
export const FESTIVAL_GREETINGS: Record<FestivalId, string> = {
  newYear: '新年快乐，万事顺意 🎉',
  newYearEve: '除夕快乐，团圆守岁 🧧',
  lantern: '元宵节快乐，甜甜蜜蜜 🏮',
  dragonBoat: '端午安康，粽叶飘香 🐉',
  qixi: '七夕快乐，浪漫满分 💫',
  midAutumn: '中秋快乐，月圆人团圆 🌕',
  doubleNinth: '重阳安康，登高望远 🍂',
  laba: '腊八节快乐，粥暖人心 🥣',
  nationalDay: '国庆快乐，假期愉快 🇨🇳',
  christmas: '圣诞快乐，平安喜乐 🎄',
}

/** 文案轮换周期（分钟）：同一窗口内稳定，跨窗口才换。 */
export const ROTATION_MINUTES = 30

/** 自 0 点起的分钟数。 */
function minutesOfDay(date: Date): number {
  return date.getHours() * 60 + date.getMinutes()
}

function isValidDate(date: Date): boolean {
  return date instanceof Date && Number.isFinite(date.getTime())
}

/** 判断时段；非法日期回退为「上午」。 */
export function greetingPeriodForDate(date: Date): GreetingPeriod {
  if (!isValidDate(date)) return 'morning'
  const minutes = minutesOfDay(date)
  let period: GreetingPeriod = 'lateNight'
  for (const bound of PERIOD_BOUNDS) {
    if (minutes >= bound.start) period = bound.key
  }
  return period
}

/** 是否周末（周六 / 周日）。 */
export function isWeekend(date: Date): boolean {
  if (!isValidDate(date)) return false
  const day = date.getDay()
  return day === 0 || day === 6
}

interface LunarParts {
  month: string
  day: number
}

/** 取农历月/日；环境不支持中文日历时返回 null。 */
function lunarParts(date: Date): LunarParts | null {
  try {
    const formatter = new Intl.DateTimeFormat('zh-CN-u-ca-chinese', {
      month: 'long',
      day: 'numeric',
    })
    const parts = formatter.formatToParts(date)
    const month = parts.find((part) => part.type === 'month')?.value ?? ''
    const day = Number(parts.find((part) => part.type === 'day')?.value ?? '')
    if (!month || !Number.isFinite(day) || day <= 0) return null
    return { month, day }
  } catch {
    return null
  }
}

/** 第二天同一时刻（用于判定除夕）。 */
function nextDay(date: Date): Date {
  const copy = new Date(date.getTime())
  copy.setDate(copy.getDate() + 1)
  return copy
}

/**
 * 命中的节日；无则返回 null。
 * 同一天同时命中多个时，公历节日优先（例如中秋恰逢 10/1）。
 */
export function festivalForDate(date: Date): FestivalId | null {
  if (!isValidDate(date)) return null
  const month = date.getMonth() + 1
  const day = date.getDate()

  if (month === 1 && day === 1) return 'newYear'
  if (month === 10 && day <= 3) return 'nationalDay'
  if (month === 12 && day === 25) return 'christmas'

  const lunar = lunarParts(date)
  if (!lunar) return null
  if (lunar.month === '正月' && lunar.day === 1) return 'newYear'
  if (lunar.month === '正月' && lunar.day === 15) return 'lantern'
  if (lunar.month === '五月' && lunar.day === 5) return 'dragonBoat'
  if (lunar.month === '七月' && lunar.day === 7) return 'qixi'
  if (lunar.month === '八月' && lunar.day === 15) return 'midAutumn'
  if (lunar.month === '九月' && lunar.day === 9) return 'doubleNinth'
  if (lunar.month === '腊月' && lunar.day === 8) return 'laba'

  const tomorrow = lunarParts(nextDay(date))
  if (tomorrow && tomorrow.month === '正月' && tomorrow.day === 1) return 'newYearEve'

  return null
}

/** 时间窗序号（用于确定性轮换）；非法日期或周期返回 0。 */
export function rotationSlot(date: Date, minutes: number = ROTATION_MINUTES): number {
  if (!isValidDate(date)) return 0
  const period = Number.isFinite(minutes) && minutes >= 1 ? Math.floor(minutes) : ROTATION_MINUTES
  return Math.floor(date.getTime() / (period * 60_000))
}

function pick<T>(items: readonly T[], seed: number): T {
  const index = ((seed % items.length) + items.length) % items.length
  return items[index]
}

/**
 * 依据时间返回问候语：节日彩蛋 > 时段文案；
 * 周末与工作日各自从自己的文案池里轮换，互不混用。
 */
export function greetingForDate(date: Date): Greeting {
  const key = greetingPeriodForDate(date)
  const weekend = isWeekend(date)
  const festival = festivalForDate(date)

  if (festival) {
    return { key, text: FESTIVAL_GREETINGS[festival], festival, weekend }
  }

  // 周末只用整套周末文案：避免「再坚持一下，快下班了」这类工作日措辞出现在周六日。
  const candidates = weekend ? WEEKEND_GREETINGS[key] : GREETINGS[key]
  return { key, text: pick(candidates, rotationSlot(date)), weekend }
}
