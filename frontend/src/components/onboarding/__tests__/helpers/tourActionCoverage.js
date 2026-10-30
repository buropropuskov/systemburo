import { parse } from 'vue/compiler-sfc';

export const AA_ACTION_OWNERS = ['src/views/AccessibleAttachmentsView.vue','src/components/AttachmentExecutionMark.vue'];

// Explicit boundary, not an inventory of the application's descendant controls.
export const AA_ACTION_COMPONENTS = {
  RefreshButton: 'command', BaseDropdown: 'command', FilterButton: 'command',
  FilterSheet: 'command', BaseModal: 'command', AccessibleAttachmentsEmptyState: 'command',
  AdminPageShell: 'presentation', AppIcon: 'presentation', Badge: 'presentation',
  StatusBadge: 'presentation', SkeletonCard: 'presentation',
  ApplicationAttachmentDetail: 'presentation', XlsxViewer: 'presentation',
  AttachmentExecutionMark: { kind:'scanned-child', events:['marked'] },
};

const ID_ATTRIBUTES = ['data-onboarding-action','data-testid','content-testid'];
const staticAttribute = (node, name) => node.props.find(prop => prop.type === 6 && prop.name === name)?.value?.content;
const eventNames = node => node.props.filter(prop => prop.type === 7 && prop.name === 'on').map(prop => prop.arg?.isStatic ? prop.arg.content : null);

/** Parse templates without executing directives, business imports or render functions. */
export function collectTemplateActions(source, { file, components = {} }) {
  const { descriptor, errors: parseErrors } = parse(source, { filename:file });
  const errors = parseErrors.map(error => `${file}: ${error.message || error}`);
  const actions = new Set();
  if (!descriptor.template?.ast) return { actions, errors:[...errors,`${file}: missing template AST`] };
  function visit(node) {
    if (node.type !== 1) { node.children?.forEach(visit); return; }
    const events = eventNames(node);
    const classification = components[node.tag];
    const kind = typeof classification === 'string' ? classification : classification?.kind;
    const where = `${file}:${node.loc.start.line} <${node.tag}>`;
    if (events.includes(null)) errors.push(`${where}: dynamic event requires explicit classification`);
    if (node.props.some(prop=>prop.type === 6 && /^on/i.test(prop.name))) errors.push(`${where}: raw event attribute is ambiguous`);
    for (const binding of node.props.filter(prop=>prop.type === 7 && prop.name === 'bind')) {
      if (binding.arg && !binding.arg.isStatic) errors.push(`${where}: dynamic bind argument is ambiguous`);
      else if (!binding.arg) errors.push(`${where}: unclassified bind spread is ambiguous`);
      else if (/^on/i.test(binding.arg.content)) errors.push(`${where}: event prop binding is ambiguous`);
    }
    if (node.tag === 'component' || (node.tagType === 1 && !kind)) errors.push(`${where}: unknown component; classify its action boundary`);
    if (kind === 'presentation' && events.length) errors.push(`${where}: presentation component gained events; classify its commands`);
    if (kind === 'scanned-child' && events.some(event => !classification.events.includes(event))) errors.push(`${where}: unclassified child event`);
    const nativeAction = node.tagType === 0 && (
      ['button','input','select','textarea'].includes(node.tag)
      || (node.tag === 'a' && node.props.some(prop => prop.name === 'href' || prop.arg?.content === 'href'))
      || events.some(event => event !== 'onboarding-retry' || !staticAttribute(node,'data-ob-owner'))
    );
    if (kind === 'command' || nativeAction) {
      const semanticId = staticAttribute(node, 'data-onboarding-action');
      const dynamicId = node.props.some(prop => prop.type === 7 && prop.name === 'bind' && ID_ATTRIBUTES.includes(prop.arg?.content));
      const id = semanticId || staticAttribute(node, 'data-testid') || staticAttribute(node, 'content-testid');
      if (dynamicId && !semanticId) errors.push(`${where}: dynamic ID needs a static data-onboarding-action`);
      else if (!id?.trim()) errors.push(`${where}: action needs a stable ID`);
      else actions.add(id);
    }
    node.children?.forEach(visit);
  }
  visit(descriptor.template.ast);
  return { actions, errors };
}

/** Every action in these owners needs a decision; owned decisions cannot become orphans. */
export function checkTourActionCoverage({ owners, entries, stepsByTour, components = {} }) {
  const errors = [];
  const actionsByOwner = new Map();
  for (const owner of owners) {
    const result = collectTemplateActions(owner.source, { file:owner.file, components });
    errors.push(...result.errors);
    actionsByOwner.set(owner.file, result.actions);
    for (const id of result.actions) {
      const entry = entries[id];
      if (!entry) { errors.push(`${owner.file}/${id}: missing coverage`); continue; }
      if (entry.owner !== owner.file) errors.push(`${id}: missing or mismatched owner`);
      const covered = typeof entry.tour === 'string';
      const skipped = typeof entry.skip === 'string' && entry.skip.trim().length > 20;
      if (covered === skipped || (covered && entry.skip !== undefined)) errors.push(`${id}: choose one valid step or reasoned skip`);
      if (covered && !stepsByTour[entry.tour]?.has(entry.step)) errors.push(`${id}: unknown tour/step ${entry.tour}/${entry.step}`);
    }
  }
  for (const [id, entry] of Object.entries(entries)) {
    if (entry.owner && !actionsByOwner.has(entry.owner)) errors.push(`${id}: unknown owner ${entry.owner}`);
    else if (entry.owner && !actionsByOwner.get(entry.owner).has(id)) errors.push(`${id}: orphan owner action`);
  }
  return errors;
}
