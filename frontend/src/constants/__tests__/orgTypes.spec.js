import { describe, it, expect } from 'vitest'
import {
  ORG_TYPES,
  ORG_TYPE_CREATE_OPTIONS,
  ORG_TYPE_DETAIL_OPTIONS,
  ORG_TYPE_FILTER_OPTIONS,
  ORG_TYPE_FILTER_ALL,
  ORG_TYPE_FILTER_UNSPECIFIED,
  orgTypeLabel,
} from '../orgTypes'

describe('константы типов справочников (#1046)', () => {
  it('4 значения типа', () => {
    expect(ORG_TYPES).toEqual(['Арендатор', 'Подрядчик', 'Отдел', 'Организация'])
  })

  it('опции создания - только 4 значения без «не указан»', () => {
    expect(ORG_TYPE_CREATE_OPTIONS).toHaveLength(4)
    expect(ORG_TYPE_CREATE_OPTIONS.some(o => o.value === null)).toBe(false)
  })

  it('опции деталей содержат «не указан» со значением null', () => {
    const unspecified = ORG_TYPE_DETAIL_OPTIONS.find(o => o.value === null)
    expect(unspecified).toBeTruthy()
    expect(unspecified.label).toBe('не указан')
  })

  it('опции фильтра: «Тип: все» + 4 + «не указан»', () => {
    expect(ORG_TYPE_FILTER_OPTIONS).toHaveLength(6)
    expect(ORG_TYPE_FILTER_OPTIONS[0].value).toBe(ORG_TYPE_FILTER_ALL)
    expect(ORG_TYPE_FILTER_OPTIONS.at(-1).value).toBe(ORG_TYPE_FILTER_UNSPECIFIED)
  })

  it('orgTypeLabel: значение как есть, пусто/NULL -> «не указан»', () => {
    expect(orgTypeLabel('Отдел')).toBe('Отдел')
    expect(orgTypeLabel(null)).toBe('не указан')
    expect(orgTypeLabel('')).toBe('не указан')
    expect(orgTypeLabel(undefined)).toBe('не указан')
  })
})
