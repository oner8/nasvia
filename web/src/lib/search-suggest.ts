/**
 * 联网搜索框的联想下拉（纯逻辑，便于单测）。
 */

/**
 * 把联想词拆成「已输入部分」和「补全部分」：与 Google 一样，补全部分加粗显示。
 * 联想词不以关键词开头（例如纠错、同义词）时整条都算补全部分。
 */
export function splitSuggestion(item: string, query: string): [typed: string, completion: string] {
  const keyword = query.trim()
  if (keyword && item.toLowerCase().startsWith(keyword.toLowerCase())) {
    return [item.slice(0, keyword.length), item.slice(keyword.length)]
  }
  return ['', item]
}

/**
 * 键盘上下选择：-1 表示「回到输入框里自己打的字」。
 * 向下从最后一条回到 -1，向上从 -1 跳到最后一条，与 Google 的行为一致。
 */
export function moveActive(current: number, delta: 1 | -1, length: number): number {
  if (length <= 0) return -1
  const next = current + delta
  if (next < -1) return length - 1
  if (next >= length) return -1
  return next
}
