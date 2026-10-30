import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { checkTourActionCoverage, collectTemplateActions, AA_ACTION_COMPONENTS, AA_ACTION_OWNERS } from './helpers/tourActionCoverage';
import coverage from '../tourCoverage.json';
import { TOURS, buildTourSteps } from '../tours';

const file = 'existing.vue';
const skip = { owner:file, skip:'Secondary action explained by the enclosing screen; no separate tour step.' };
const stepsByTour = { guard:new Set(['detail']) };
const check = (template, entries = {}, components = {}) => checkTourActionCoverage({
  owners:[{ file,source:`<template>${template}</template>` }], entries, stepsByTour, components,
});

describe('bounded template action coverage', () => {
  it('rejects a new action on an existing screen without a registry decision', () => {
    const result = checkTourActionCoverage({
      owners: [{ file:'existing.vue', source:'<template><button data-testid="aa-new" @click="send">Send</button></template>' }],
      entries:{}, stepsByTour:{},
    });
    expect(result).toEqual([expect.stringContaining('aa-new: missing coverage')]);
  });
  it('rejects a new native button without an ID, even without an explicit click handler', () => {
    expect(check('<button>Send</button>')).toEqual([expect.stringContaining('action needs a stable ID')]);
  });
  it('fails closed for pointer events rather than silently treating them as presentation', () => {
    expect(check('<div @pointerup="send"/>')).toEqual([expect.stringContaining('action needs a stable ID')]);
    expect(check('<div data-testid="new" @pointerup="send"/>')).toEqual([expect.stringContaining('new: missing coverage')]);
  });
  it.each(['v-bind="{ onClick: send }"',':\u005bkey\u005d="send"','v-on="handlers"'])('rejects ambiguous event/binding form: %s', attributes => {
    expect(check(`<div ${attributes}/>`)).not.toEqual([]);
  });
  it('rejects static event props while keeping explicit data attributes', () => {
    expect(check('<div :onClick="send"/>')).toContainEqual(expect.stringContaining('event prop binding'));
    expect(check('<div data-testid="new" :onclick="send"/>',{ new:skip })).toContainEqual(expect.stringContaining('event prop binding'));
    expect(check('<div :data-ob-state="state" :data-ob-blank="blank"/>')).toEqual([]);
  });
  it.each(['onclick','onClick'])('rejects raw %s attributes regardless of an existing action ID', attribute => {
    expect(check(`<div ${attribute}="send()"/>`)).toContainEqual(expect.stringContaining('raw event attribute'));
    expect(check(`<div data-testid="new" ${attribute}="send()"/>`,{ new:skip })).toContainEqual(expect.stringContaining('raw event attribute'));
  });
  it.each(['button','a','form','input','select','textarea','div'])('detects native %s commands', tag => {
    const source = `<${tag} data-testid="new" @${tag === 'form' ? 'submit' : 'click'}="send"></${tag}>`;
    expect(check(source)).toEqual([expect.stringContaining('new: missing coverage')]);
  });
  it('accepts a real step or a reasoned skip without executing template expressions', () => {
    const source = '<button data-testid="new" @click="throwIfExecuted()">Send</button>';
    expect(check(source,{ new:skip })).toEqual([]);
    expect(check(source,{ new:{ owner:file,tour:'guard',step:'detail' } })).toEqual([]);
  });
  it.each([
    { owner:file,tour:'missing',step:'detail' },
    { owner:file,tour:'guard',step:'missing' },
  ])('rejects an invalid tour/step reference: %j', entry => {
    expect(check('<button data-testid="new"/>',{ new:entry })).toEqual([expect.stringContaining('unknown tour/step')]);
  });
  it.each([{ owner:file,skip:'short' },{ ...skip,tour:'guard',step:'detail' }])('rejects an invalid decision: %j', entry => {
    expect(check('<button data-testid="new"/>',{ new:entry })).toEqual([expect.stringContaining('choose one valid step or reasoned skip')]);
  });
  it('rejects an action removed from its owner and an unknown owner', () => {
    expect(check('<div/>',{ removed:skip })).toEqual([expect.stringContaining('removed: orphan owner action')]);
    expect(check('<div/>',{ removed:{ ...skip,owner:'other.vue' } })).toEqual([expect.stringContaining('unknown owner')]);
  });
  it('rejects an existing action assigned to the wrong owner', () => {
    expect(check('<button data-testid="new"/>',{ new:{ ...skip,owner:'other.vue' } })).toContainEqual(expect.stringContaining('mismatched owner'));
  });
  it('keeps v-if and v-for actions without expanding the rows', () => {
    const result = collectTemplateActions('<template><button v-if="false" data-testid="conditional"/><button v-for="row in rows" :key="row.id" data-testid="row"/></template>',{ file });
    expect(result.errors).toEqual([]);
    expect([...result.actions]).toEqual(['conditional','row']);
    expect(check('<button v-if="false" data-testid="conditional"/>')).toContainEqual(expect.stringContaining('conditional: missing coverage'));
  });
  it('rejects dynamic IDs until a separate stable semantic ID is declared', () => {
    expect(check('<button :data-testid="row.id"/>')).toEqual([expect.stringContaining('dynamic ID')]);
    expect(check('<button :data-testid="row.id" data-onboarding-action="row"/>',{ row:skip })).toEqual([]);
    expect(check('<button :data-onboarding-action="row.id" data-testid="row"/>',{ row:skip })).toContainEqual(expect.stringContaining('dynamic ID'));
  });
  it('does not ignore unknown or dynamically selected components', () => {
    expect(check('<NewCommand data-testid="new"/>')).toEqual([expect.stringContaining('unknown component')]);
    expect(check('<component :is="command" data-testid="new"/>')).toContainEqual(expect.stringContaining('unknown component'));
  });
  it('classifies command components and modal content IDs explicitly', () => {
    expect(check('<RefreshButton data-testid="refresh" @refresh="reload"/>',{ refresh:skip },AA_ACTION_COMPONENTS)).toEqual([]);
    expect(check('<BaseModal content-testid="modal" @close="close"/>',{ modal:skip },AA_ACTION_COMPONENTS)).toEqual([]);
    expect(check('<RefreshButton @refresh="reload"/>',{},AA_ACTION_COMPONENTS)).toContainEqual(expect.stringContaining('stable ID'));
  });
  it('rejects new events on presentation or scanned-child boundaries and dynamic events', () => {
    expect(check('<Badge @click="send"/>',{},AA_ACTION_COMPONENTS)).toContainEqual(expect.stringContaining('presentation component gained events'));
    expect(check('<AttachmentExecutionMark @marked="refresh"/>',{},AA_ACTION_COMPONENTS)).toEqual([]);
    expect(check('<AttachmentExecutionMark @new-command="send"/>',{},AA_ACTION_COMPONENTS)).toContainEqual(expect.stringContaining('unclassified child event'));
    expect(check('<button data-testid="new" @[event]="send"/>',{ new:skip })).toContainEqual(expect.stringContaining('dynamic event'));
  });
  it('does not accept missing or malformed templates as empty coverage', () => {
    expect(checkTourActionCoverage({ owners:[{ file,source:'<script>export default {}</script>' }],entries:{},stepsByTour })).toContainEqual(expect.stringContaining('missing template AST'));
    expect(check('<button>')).not.toEqual([]);
  });
});

// Deliberately bounded: child internals, render functions, old-handler semantics,
// and screens outside these two owners still require review.
describe('AA action owners — real source and existing tour registry', () => {
  it('requires a decision for every action in both templates, with no owned orphan', () => {
    const owners = AA_ACTION_OWNERS
      .map(file => ({ file,source:readFileSync(resolve(file),'utf8') }));
    const stepsByTour = Object.fromEntries(TOURS.map(tour => [tour.key,new Set(buildTourSteps(tour,{ factTableRoute:'/table/selector-lock' }).map(step => step.id))]));
    expect(checkTourActionCoverage({ owners,entries:coverage.elements,stepsByTour,components:AA_ACTION_COMPONENTS })).toEqual([]);
  });
});
