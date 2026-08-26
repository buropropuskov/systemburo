import { describe, it, expect } from 'vitest'
import { mapWithConcurrency } from '../mapWithConcurrency'

function deferred() {
  let resolve
  const promise = new Promise((r) => { resolve = r })
  return { promise, resolve }
}

describe('mapWithConcurrency', () => {
  it('сохраняет порядок результатов по входу', async () => {
    const items = [1, 2, 3, 4, 5]
    const out = await mapWithConcurrency(items, 2, async (n) => n * 10)
    expect(out).toEqual([10, 20, 30, 40, 50])
  })

  it('держит не более limit промисов в полёте одновременно', async () => {
    const total = 7
    const limit = 3
    let inFlight = 0
    let maxInFlight = 0
    const gates = Array.from({ length: total }, () => deferred())

    const p = mapWithConcurrency(Array.from({ length: total }, (_, i) => i), limit, async (i) => {
      inFlight += 1
      maxInFlight = Math.max(maxInFlight, inFlight)
      await gates[i].promise
      inFlight -= 1
      return i
    })

    // Даём воркерам стартовать и упереться в gate.
    await Promise.resolve()
    await Promise.resolve()
    expect(maxInFlight).toBe(limit)

    // Отпускаем по одному - освободившийся слот берёт следующий элемент.
    for (let i = 0; i < total; i += 1) gates[i].resolve()
    const out = await p
    expect(out).toEqual([0, 1, 2, 3, 4, 5, 6])
    expect(maxInFlight).toBe(limit)
  })

  it('пробрасывает ошибку mapper (как Promise.all)', async () => {
    await expect(
      mapWithConcurrency([1, 2, 3], 2, async (n) => {
        if (n === 2) throw new Error('boom')
        return n
      })
    ).rejects.toThrow('boom')
  })

  it('пустой вход -> пустой результат, mapper не вызывается', async () => {
    let calls = 0
    const out = await mapWithConcurrency([], 4, async () => { calls += 1 })
    expect(out).toEqual([])
    expect(calls).toBe(0)
  })

  it('limit больше числа элементов не ломает порядок', async () => {
    const out = await mapWithConcurrency([1, 2], 10, async (n) => n + 1)
    expect(out).toEqual([2, 3])
  })
})
