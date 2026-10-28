import { beforeEach, describe, expect, it } from 'vitest';
import { readProgress, resolveStepIndex, saveProgress } from '../tourProgress';

const steps = (...ids) => ids.map(id => ({ id }));
beforeEach(() => localStorage.clear());

describe('semantic progress identity', () => {
  it('resumes the same id after previous chapters are filtered out', () => {
    saveProgress(7, 'user', 3, true, steps('a', 'b', 'c', 'd', 'e'));
    expect(readProgress(7, 'user', steps('c', 'd', 'e'))).toEqual({ index: 1, interrupted: true });
  });
  it('keeps the id after a new preceding chapter is inserted', () => {
    saveProgress(7, 'user', 1, false, steps('a', 'b', 'c'));
    expect(readProgress(7, 'user', steps('new', 'a', 'b', 'c')).index).toBe(2);
  });
  it('chooses the next surviving historical neighbour when the current step becomes unavailable', () => {
    expect(resolveStepIndex('c', ['a', 'b', 'c', 'd', 'e'], steps('new', 'a', 'e'), 2)).toBe(2);
  });
  it('chooses a previous neighbour when no later historical step remains', () => {
    expect(resolveStepIndex('d', ['a', 'b', 'c', 'd'], steps('a', 'c'), 3)).toBe(1);
  });
  it('starts at the beginning when no semantic neighbour survives', () => {
    expect(resolveStepIndex('d', ['a', 'b', 'c', 'd'], steps('new', 'end'), 3)).toBe(0);
  });
  it('retains a bounded compatibility path for index-only records without inventing an id', () => {
    saveProgress(7, 'user', 6, true);
    expect(readProgress(7, 'user', steps('a', 'b', 'c')).index).toBe(2);
    expect(JSON.parse(localStorage.getItem('ob:progress:7:user'))).not.toHaveProperty('stepId');
  });
  it('writes identity additively without changing the tour version or completion state', () => {
    saveProgress(7, 'user', 1, true);
    const previous = JSON.parse(localStorage.getItem('ob:progress:7:user'));
    saveProgress(7, 'user', 1, true, steps('a', 'b', 'c'));
    expect(JSON.parse(localStorage.getItem('ob:progress:7:user'))).toMatchObject({
      version: previous.version, index: 1, onScreen: true, stepId: 'b', stepIds: ['a', 'b', 'c'],
    });
  });
});
