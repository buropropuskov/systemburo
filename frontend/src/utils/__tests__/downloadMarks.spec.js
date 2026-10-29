import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { downloadMarks, downloadMarkKey } from '../downloadMarks';
beforeEach(()=>{vi.useFakeTimers();vi.setSystemTime(new Date('2040-01-01T12:00:00Z'));downloadMarks.setOwner(null);downloadMarks.setOwner('1/0');});
afterEach(()=>{downloadMarks.setOwner(null);vi.useRealTimers();});
describe('temporary download marks',()=>{
  it('expires after five minutes and uses just one timer for multiple files',()=>{
    downloadMarks.mark('one',downloadMarks.lease());vi.advanceTimersByTime(1000);downloadMarks.mark('two',downloadMarks.lease());
    expect(vi.getTimerCount()).toBe(1);vi.advanceTimersByTime(299000);
    expect(downloadMarks.has('one')).toBe(false);expect(downloadMarks.has('two')).toBe(true);
    vi.advanceTimersByTime(1000);expect(downloadMarks.has('two')).toBe(false);expect(vi.getTimerCount()).toBe(0);
  });
  it('clears another account and ignores a late download even after switching back',()=>{
    const lease=downloadMarks.lease();downloadMarks.mark('old',lease);
    downloadMarks.setOwner('2/0');expect(downloadMarks.has('old')).toBe(false);
    downloadMarks.setOwner('1/0');downloadMarks.mark('late',lease);expect(downloadMarks.has('late')).toBe(false);
  });
  it('does not share status across files, applications or a changed file version',()=>{
    const file={id:3,file_name:'guide.pdf',file_size:100};
    const key=downloadMarkKey(1,file);downloadMarks.mark(key,downloadMarks.lease());
    expect(downloadMarks.has(downloadMarkKey(2,file))).toBe(false);
    expect(downloadMarks.has(downloadMarkKey(1,{...file,file_size:101}))).toBe(false);
    expect(downloadMarks.has(key)).toBe(true);
  });
});
