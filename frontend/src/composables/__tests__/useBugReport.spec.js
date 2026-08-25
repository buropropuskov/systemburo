import { describe, it, expect } from 'vitest'
import { buildBugContext } from '@/composables/useBugReport'

describe('composables/useBugReport - buildBugContext', () => {
  it('запоминает адрес страницы, с которой ушёл упавший запрос', () => {
    const ctx = buildBugContext({
      route: 'GET /applications',
      httpStatus: 502,
      message: 'Bad Gateway',
      uiRoute: '/center?status=new',
    })
    expect(ctx.uiRoute).toBe('/center?status=new')
    expect(ctx.httpStatus).toBe(502)
  })

  it('без адреса страницы отдаёт пустую строку, а не undefined', () => {
    expect(buildBugContext({ route: 'GET /news', httpStatus: 500 }).uiRoute).toBe('')
  })

  it('режет длинный адрес страницы так же, как маршрут запроса', () => {
    const ctx = buildBugContext({ route: 'GET /x', httpStatus: 500, uiRoute: '/c?q=' + 'a'.repeat(400) })
    expect(ctx.uiRoute).toHaveLength(255)
  })
})
