import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LoaderSpinner from '../LoaderSpinner.vue'

describe('LoaderSpinner', () => {
  it('renders default label "Загрузка…"', () => {
    const wrapper = mount(LoaderSpinner)
    expect(wrapper.text()).toContain('Загрузка…')
  })

  it('renders custom label', () => {
    const wrapper = mount(LoaderSpinner, { props: { label: 'Подождите' } })
    expect(wrapper.text()).toContain('Подождите')
  })

  it('omits label when empty', () => {
    const wrapper = mount(LoaderSpinner, { props: { label: '' } })
    expect(wrapper.find('.loader-spinner__label').exists()).toBe(false)
  })

  it('applies size modifier', () => {
    const wrapper = mount(LoaderSpinner, { props: { size: 'small' } })
    expect(wrapper.classes()).toContain('loader-spinner--small')
  })

  it('applies inline modifier', () => {
    const wrapper = mount(LoaderSpinner, { props: { inline: true } })
    expect(wrapper.classes()).toContain('loader-spinner--inline')
  })

  it('has role="status" for a11y', () => {
    const wrapper = mount(LoaderSpinner)
    expect(wrapper.attributes('role')).toBe('status')
  })

  it('rejects invalid size via validator', () => {
    const validator = LoaderSpinner.props.size.validator
    expect(validator('small')).toBe(true)
    expect(validator('huge')).toBe(false)
  })
})
