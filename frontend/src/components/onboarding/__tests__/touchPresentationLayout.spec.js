import { describe, expect, it } from 'vitest';
import { touchPresentationLayout } from '../touchPresentationLayout';
const target = (left, top, width, height) => ({ left, top, width, height, right: left + width, bottom: top + height });
describe('touch sizing preserves Driver placement ownership', () => {
  it('keeps a mouse desktop in the full-text presentation', () => {
    expect(touchPresentationLayout({ width: 1440, height: 900 })).toEqual({ touch: false });
    expect(touchPresentationLayout({ width: 1024, height: 768 })).toEqual({ touch: false });
  });
  it.each([[390,844,358], [768,1024,400], [1024,768,976]])('sizes %s x %s within its existing viewport', (width,height,expected) => {
    const result=touchPresentationLayout({width,height,coarse:true});
    expect(result.width).toBe(expected); expect(result.maxHeight).toBeLessThan(height); expect(result.fits).toBe(true);
  });
  it('uses the larger vertical gap when a sheet occupies the lower half', () => {
    const result=touchPresentationLayout({width:390,height:844,target:target(0,470,390,374),coarse:true});
    expect(result.maxHeight).toBe(438); expect(result.fits).toBe(true);
    expect(result).not.toHaveProperty('top'); expect(result).not.toHaveProperty('left');
  });
  it('uses the visual viewport offsets rather than assuming a full-height screen', () => {
    const result=touchPresentationLayout({width:390,height:400,top:100,left:0,target:target(0,200,390,50),coarse:true});
    expect(result.maxHeight).toBe(218);
  });
  it('identifies impossible free space instead of reporting a successful fit', () => {
    const result=touchPresentationLayout({width:390,height:300,target:target(0,0,390,300),coarse:true});
    expect(result.fits).toBe(false);
  });
  it('allows a side gap to use the full available height', () => {
    const result=touchPresentationLayout({width:768,height:1024,target:target(0,200,200,700),coarse:true});
    expect(result.maxHeight).toBe(976);
  });
  it.each([target(420,100,300,700), target(48,100,300,700)])('reserves both viewport gutter and target gap near either side threshold', box => {
    const result=touchPresentationLayout({width:768,height:1024,target:box,coarse:true});
    expect(result.maxHeight).toBe(184);
  });
  it('measures side space relative to visual viewport offset', () => {
    const result=touchPresentationLayout({width:768,height:1024,left:100,top:50,target:target(520,150,300,700),coarse:true});
    expect(result.maxHeight).toBe(184);
  });
});
