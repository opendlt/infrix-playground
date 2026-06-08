// Infrix Playground — portable bundle → Cinema proof scene (adoption-09).
//
// Pure, node-independent: turns a proof receipt + the portable evidence
// package into a Cinema `proof` object (a SceneGraph plus an assurance label)
// that cinema-core's `cinema.proof` mode renders. The scene is the governed
// flow's own structure — intent → plan → policy → approval → credential →
// outcome → anchor — so "Watch a replay" shows a real, non-blank picture of the
// proof, with no node trust and no live polling.

const FLOW = [
  { id: 'intent', label: 'Intent', kind: 'intent' },
  { id: 'plan', label: 'Plan', kind: 'plan_step' },
  { id: 'policy', label: 'Policy: allow', kind: 'policy' },
  { id: 'approval', label: 'Operator approval', kind: 'approval' },
  { id: 'credential', label: 'Delivery credential', kind: 'external_proof' },
  { id: 'outcome', label: 'Outcome: released', kind: 'outcome' },
  { id: 'anchor', label: 'Anchored', kind: 'anchor' },
];

const COLORS = {
  intent: { r: 100, g: 181, b: 246, a: 220 },
  plan_step: { r: 129, g: 199, b: 132, a: 220 },
  policy: { r: 255, g: 213, b: 79, a: 220 },
  approval: { r: 186, g: 104, b: 200, a: 220 },
  external_proof: { r: 77, g: 208, b: 225, a: 220 },
  outcome: { r: 129, g: 199, b: 132, a: 240 },
  anchor: { r: 240, g: 98, b: 146, a: 220 },
};

/**
 * bundleToCinemaProof builds the cinema-core `proof` object from a receipt and
 * (optionally) the portable package. `anchored` is taken from the package when
 * present, else from the receipt's anchor artifact.
 */
export function bundleToCinemaProof(receipt, pkg) {
  const r = receipt || {};
  const a = r.assurance || {};
  const art = r.artifacts || {};
  const anchored = !!(pkg && pkg.anchorTxHash) || !!art.anchorTx;

  const steps = FLOW.filter((s) => s.id !== 'anchor' || anchored);
  const nodes = steps.map((s, i) => ({
    id: s.id,
    kind: s.kind,
    label: s.label,
    position: { x: i * 130, y: (i % 2) * 70 },
    size: 16,
    color: COLORS[s.kind] || COLORS.plan_step,
    shape: s.kind === 'anchor' ? 'diamond' : 'rectangle',
    createdAtEvent: i,
    lastUpdated: i,
  }));
  const edges = [];
  for (let i = 1; i < steps.length; i++) {
    edges.push({ from: steps[i - 1].id, to: steps[i].id, kind: 'flow' });
  }

  const label = a.label || a.proofLevel || (anchored ? 'L4' : 'L3');
  return {
    scene: { nodes, edges },
    assurance: {
      id: a.l0Verified ? 'l0_confirmed' : 'offline',
      label,
      proofLevel: a.proofLevel || (anchored ? 'L4' : 'L3'),
      l0Verified: !!a.l0Verified,
      nodeTrusted: a.nodeTrusted === true,
    },
    details: {
      intent: art.intentId || '',
      plan: art.planId || '',
      outcome: art.outcomeId || '',
      evidence: art.evidenceId || '',
      anchor: art.anchorTx || '',
    },
    meta: { subject: r.subject || {}, status: r.status || '' },
  };
}

/**
 * sceneNodeCount is a tiny helper the smoke test uses to assert a non-blank
 * scene without reaching into the structure.
 */
export function sceneNodeCount(proof) {
  return proof && proof.scene && Array.isArray(proof.scene.nodes) ? proof.scene.nodes.length : 0;
}
