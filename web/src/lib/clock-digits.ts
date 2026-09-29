/** 时刻拆分：把 Date 拆成两位的时/分/秒，供时钟展示与转筒数字使用。 */

export interface ClockParts {
  /** 两位小时，如 "09"。 */
  hours: string
  /** 两位分钟，如 "57"。 */
  minutes: string
  /** 两位秒，如 "05"。 */
  seconds: string
}

const pad = (value: number): string => String(Math.floor(value)).padStart(2, '0')

/** 按本地时间拆分时刻；非法日期回退为 00:00:00。 */
export function clockParts(date: Date): ClockParts {
  const time = date instanceof Date ? date.getTime() : Number.NaN
  if (!Number.isFinite(time)) {
    return { hours: '00', minutes: '00', seconds: '00' }
  }
  return {
    hours: pad(date.getHours()),
    minutes: pad(date.getMinutes()),
    seconds: pad(date.getSeconds()),
  }
}

/** 供读屏使用的可读时间，例如「16点57分05秒」。 */
export function clockLabel(date: Date): string {
  const { hours, minutes, seconds } = clockParts(date)
  return `${Number(hours)}点${Number(minutes)}分${Number(seconds)}秒`
}
