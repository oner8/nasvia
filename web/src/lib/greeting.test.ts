import test from 'node:test'
import assert from 'node:assert/strict'

import {
  FESTIVAL_GREETINGS,
  GREETINGS,
  ROTATION_MINUTES,
  WEEKEND_GREETINGS,
  festivalForDate,
  greetingForDate,
  greetingPeriodForDate,
  isWeekend,
  rotationSlot,
} from './greeting.ts'

/** 构造本地时间（避免时区换算）。 */
const at = (year: number, month: number, day: number, hour = 0, minute = 0) =>
  new Date(year, month - 1, day, hour, minute, 0)

/** 全天每 5 分钟采样一次。 */
function samplesOf(day: Date) {
  const out: Array<{ label: string; date: Date; greeting: ReturnType<typeof greetingForDate> }> = []
  for (let hour = 0; hour < 24; hour += 1) {
    for (let minute = 0; minute < 60; minute += 5) {
      const date = new Date(day.getFullYear(), day.getMonth(), day.getDate(), hour, minute)
      out.push({
        label: `${hour}:${String(minute).padStart(2, '0')}`,
        date,
        greeting: greetingForDate(date),
      })
    }
  }
  return out
}

const WORK_WORDS = /下班|上班|加班|例会|同事/

test('时段边界：深夜与上午（分钟级）', () => {
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 4, 59)), 'lateNight')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 5, 0)), 'dawn')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 6, 59)), 'dawn')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 7, 0)), 'earlyMorning')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 8, 59)), 'earlyMorning')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 9, 0)), 'morning')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 11, 59)), 'morning')
})

test('时段边界：按上班节奏（午休 → 下午上班 → 收尾 → 下班后）', () => {
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 12, 0)), 'noon')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 13, 59)), 'noon')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 14, 0)), 'afternoon')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 16, 29)), 'afternoon')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 16, 30)), 'wrapUp')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 17, 29)), 'wrapUp')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 17, 30)), 'dusk')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 18, 59)), 'dusk')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 19, 0)), 'evening')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 21, 59)), 'evening')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 22, 0)), 'lateNight')
  assert.equal(greetingPeriodForDate(at(2026, 9, 28, 23, 59)), 'lateNight')
})

test('回归：13:36 是午休，工作日与周末都不出现工作收尾话术', () => {
  const workday = greetingForDate(at(2026, 9, 28, 13, 36))
  assert.equal(workday.key, 'noon')
  assert.ok(GREETINGS.noon.includes(workday.text), `工作日 13:36 应取午休文案：${workday.text}`)
  assert.ok(!WORK_WORDS.test(workday.text), `午休不该出现工作话术：${workday.text}`)

  const sunday = greetingForDate(at(2026, 9, 27, 13, 36))
  assert.equal(sunday.key, 'noon')
  assert.equal(sunday.weekend, true)
  assert.ok(WEEKEND_GREETINGS.noon.includes(sunday.text), `周末 13:36 应取周末午休文案：${sunday.text}`)
  assert.ok(!WORK_WORDS.test(sunday.text), `周末不该出现工作话术：${sunday.text}`)
})

test('「快下班」只出现在 16:30–17:30，16:30 之前不出现「下班」字样', () => {
  for (const day of [26, 28]) {
    // 周六 与 周一
    for (let hour = 0; hour < 24; hour += 1) {
      for (const minute of [0, 15, 29, 30, 45]) {
        const date = at(2026, 9, day, hour, minute)
        const { key, text, weekend } = greetingForDate(date)
        const label = `2026-09-${day} ${hour}:${String(minute).padStart(2, '0')}`
        if (text.includes('快下班')) {
          assert.equal(key, 'wrapUp', `${label} 出现「快下班」但时段是 ${key}`)
        }
        if (hour * 60 + minute < 16 * 60 + 30) {
          assert.ok(!text.includes('下班'), `${label} 16:30 之前不该出现「下班」：${text}`)
        }
        // 周末任何时刻都不该出现工作日词
        if (weekend) {
          assert.ok(!WORK_WORDS.test(text), `${label} 周末不该出现工作话术：${text}`)
        }
      }
    }
  }
})

test('周末只用周末文案、工作日只用工作日文案（全天采样）', () => {
  const saturday = at(2026, 9, 26)
  const sunday = at(2026, 9, 27)
  const monday = at(2026, 9, 28)
  const wednesday = at(2026, 9, 30)

  assert.equal(isWeekend(saturday), true)
  assert.equal(isWeekend(sunday), true)
  assert.equal(isWeekend(monday), false)
  assert.equal(isWeekend(wednesday), false)
  assert.equal(greetingForDate(saturday).weekend, true)
  assert.equal(greetingForDate(monday).weekend, false)

  for (const day of [saturday, sunday]) {
    for (const { label, greeting } of samplesOf(day)) {
      if (greeting.festival) continue
      assert.ok(
        WEEKEND_GREETINGS[greeting.key].includes(greeting.text),
        `${day.toDateString()} ${label} 周末文案必须来自周末池（${greeting.key}）：${greeting.text}`,
      )
      assert.ok(!WORK_WORDS.test(greeting.text), `${label} 周末不该出现工作话术：${greeting.text}`)
    }
  }

  for (const day of [monday, wednesday]) {
    for (const { label, greeting } of samplesOf(day)) {
      if (greeting.festival) continue
      assert.ok(
        GREETINGS[greeting.key].includes(greeting.text),
        `${day.toDateString()} ${label} 工作日文案必须来自工作日池（${greeting.key}）：${greeting.text}`,
      )
    }
  }
})

test('周末文案与工作日文案没有任何重叠', () => {
  const weekendLines = new Set(Object.values(WEEKEND_GREETINGS).flat())
  for (const lines of Object.values(GREETINGS)) {
    for (const line of lines) {
      assert.ok(!weekendLines.has(line), `两条池子不应重复：${line}`)
    }
  }
})

test('节日：公历（元旦 / 国庆 / 圣诞）', () => {
  assert.equal(festivalForDate(at(2026, 1, 1)), 'newYear')
  assert.equal(festivalForDate(at(2026, 10, 1)), 'nationalDay')
  assert.equal(festivalForDate(at(2026, 10, 3)), 'nationalDay')
  assert.equal(festivalForDate(at(2026, 10, 4)), null)
  assert.equal(festivalForDate(at(2026, 12, 25)), 'christmas')
})

test('节日：农历（春节 / 除夕 / 元宵 / 端午 / 七夕 / 中秋 / 重阳 / 腊八）', () => {
  assert.equal(festivalForDate(at(2026, 2, 17)), 'newYear') // 正月初一
  assert.equal(festivalForDate(at(2026, 2, 16)), 'newYearEve') // 次日为正月初一
  assert.equal(festivalForDate(at(2026, 3, 3)), 'lantern') // 正月十五
  assert.equal(festivalForDate(at(2026, 6, 19)), 'dragonBoat') // 五月初五
  assert.equal(festivalForDate(at(2026, 8, 19)), 'qixi') // 七月初七
  assert.equal(festivalForDate(at(2026, 9, 25)), 'midAutumn') // 八月十五
  assert.equal(festivalForDate(at(2026, 10, 18)), 'doubleNinth') // 九月初九
  assert.equal(festivalForDate(at(2026, 1, 26)), 'laba') // 腊月初八
})

test('节日：普通日子不命中，节日当天展示彩蛋文案', () => {
  assert.equal(festivalForDate(at(2026, 5, 20)), null)
  const greeting = greetingForDate(at(2026, 2, 17, 20, 5))
  assert.equal(greeting.festival, 'newYear')
  assert.equal(greeting.text, FESTIVAL_GREETINGS.newYear)
  // 节日当天不分时段都用彩蛋文案
  assert.equal(greetingForDate(at(2026, 2, 17, 7, 0)).text, FESTIVAL_GREETINGS.newYear)
})

test('轮换：同一时间窗内稳定，跨窗口会更换措辞', () => {
  const base = at(2026, 5, 20, 10, 0)
  const sameSlot = new Date(base.getTime() + 5 * 60_000)
  assert.equal(
    greetingForDate(base).text,
    greetingForDate(sameSlot).text,
    '同一 30 分钟窗口内文案应一致',
  )
  assert.equal(rotationSlot(base, ROTATION_MINUTES), rotationSlot(sameSlot, ROTATION_MINUTES))

  const texts = new Set<string>()
  for (let offset = 0; offset < 6 * ROTATION_MINUTES; offset += ROTATION_MINUTES) {
    texts.add(greetingForDate(new Date(base.getTime() + offset * 60_000)).text)
  }
  assert.ok(texts.size > 1, '跨多个窗口应出现不同文案')
})

test('健壮性：非法日期不抛错，所有时段都有文案', () => {
  const invalid = new Date(Number.NaN)
  assert.equal(greetingPeriodForDate(invalid), 'morning')
  assert.equal(festivalForDate(invalid), null)
  assert.equal(isWeekend(invalid), false)
  assert.equal(rotationSlot(invalid), 0)
  const invalidGreeting = greetingForDate(invalid)
  assert.equal(typeof invalidGreeting.text, 'string')
  assert.ok(invalidGreeting.text.length > 0)

  const hoursByPeriod: Record<string, number> = {
    lateNight: 23,
    dawn: 5,
    earlyMorning: 7,
    morning: 9,
    noon: 12,
    afternoon: 14,
    wrapUp: 17,
    dusk: 18,
    evening: 19,
  }
  for (const [period, hour] of Object.entries(hoursByPeriod)) {
    for (const day of [26, 28]) {
      // 周六 与 周一
      const greeting = greetingForDate(at(2026, 9, day, hour, 0))
      assert.equal(greeting.key, period, `${period} 应落在 ${hour} 点`)
      assert.ok(greeting.text.trim().length > 0, `${period} 文案不应为空`)
    }
  }
  // 每个时段的两套文案都不少于 3 条
  for (const lines of [...Object.values(GREETINGS), ...Object.values(WEEKEND_GREETINGS)]) {
    assert.ok(lines.length >= 3, `每个时段至少 3 条文案，实际 ${lines.length}`)
  }
})
