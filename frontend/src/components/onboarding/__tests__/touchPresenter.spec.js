import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createTouchPresenter } from '../touchPresenter';
let presenter, alive, refresh, popover;
const originalWidth=window.innerWidth, originalHeight=window.innerHeight;
function box(width,height){return {left:0,top:0,right:width,bottom:height,width,height};}
beforeEach(()=>{
 window.innerWidth=390;window.innerHeight=844;alive=true;refresh=vi.fn();
 const wrapper=document.createElement('div');wrapper.className='driver-popover ob-popover';
 const title=document.createElement('h2'),description=document.createElement('div'),footerButtons=document.createElement('div');
 description.innerHTML='<p>Первое объяснение.</p><p>Полный текст с <strong>важным условием</strong> сохранён.</p>';
 wrapper.append(title,description,footerButtons);document.body.append(wrapper);
 wrapper.getBoundingClientRect=()=>box(358,230);description.getBoundingClientRect=()=>box(320,60);
 popover={wrapper,title,description,footerButtons};
 presenter=createTouchPresenter({getDriver:()=>({refresh}),isAlive:()=>alive});
});
afterEach(()=>{presenter?.detach();document.body.replaceChildren();window.innerWidth=originalWidth;window.innerHeight=originalHeight;vi.restoreAllMocks();});
describe('real Driver touch presentation lease',()=>{
 it('offers More only for genuinely hidden content, while retaining access to a demo',()=>{
  Object.defineProperty(popover.description,'clientHeight',{configurable:true,value:40});
  Object.defineProperty(popover.description,'scrollHeight',{configurable:true,value:40});
  presenter.attach(popover,()=>null);
  expect(popover.wrapper.querySelector('[data-testid="ob-touch-details"]').hidden).toBe(true);
  Object.defineProperty(popover.description,'scrollHeight',{configurable:true,value:200});presenter.update();
  expect(popover.wrapper.querySelector('[data-testid="ob-touch-details"]').hidden).toBe(false);
 });
 it('does not override the hidden phase owned by Driver',()=>{
  popover.wrapper.style.display='none';presenter.attach(popover,()=>null);
  expect(popover.wrapper.style.display).toBe('none');
  popover.wrapper.style.display='block';presenter.update();expect(popover.wrapper.style.display).toBe('flex');
 });
 it('keeps full original HTML and toggles details without navigating',()=>{
  const original=popover.description.innerHTML;presenter.attach(popover,()=>null);
  const button=popover.wrapper.querySelector('[data-testid="ob-touch-details"]');
  expect(button.hidden).toBe(false);button.click();
  expect(popover.wrapper.classList.contains('ob-touch--expanded')).toBe(true);
  expect(button.textContent).toBe('Свернуть');expect(button.getAttribute('aria-expanded')).toBe('true');
  expect(popover.description.innerHTML).toBe(original);
  button.click();expect(popover.wrapper.classList.contains('ob-touch--expanded')).toBe(false);
 });
 it('preserves existing Next/Back buttons and restores desktop full text on rotation',()=>{
  const next=document.createElement('button');next.textContent='Далее';popover.footerButtons.append(next);
  presenter.attach(popover,()=>null);window.innerWidth=1440;presenter.update();
  expect(popover.wrapper.classList.contains('ob-touch')).toBe(false);
  expect(popover.wrapper.querySelector('[data-testid="ob-touch-details"]').hidden).toBe(true);
  expect(popover.footerButtons.contains(next)).toBe(true);
 });
 it('exposes one native demo frame while keeping the original figure intact',()=>{
  popover.description.innerHTML+='<figure class="ob-popover__demo"><img alt="Учебный пример" src="/fixture.png"></figure>';
  const original=popover.description.innerHTML;presenter.attach(popover,()=>null);
  expect(popover.wrapper.querySelector('.ob-touch__demo img').alt).toBe('Учебный пример');
  popover.wrapper.querySelector('[data-testid="ob-touch-details"]').click();
  expect(popover.wrapper.querySelector('.ob-touch__demo').hidden).toBe(true);
  expect(popover.description.innerHTML).toBe(original);
 });
 it('cleans listeners/frames and never revives presentation after cancellation',()=>{
  presenter.attach(popover,()=>null);alive=false;presenter.detach();
  expect(popover.wrapper.querySelector('[data-testid="ob-touch-details"]')).toBe(null);
  window.dispatchEvent(new Event('resize'));presenter.attach(popover,()=>null);
  expect(popover.wrapper.querySelector('[data-testid="ob-touch-details"]')).toBe(null);
 });
});
