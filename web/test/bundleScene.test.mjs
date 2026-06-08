// adoption-09 — playground bundle→Cinema scene logic (no browser).
//
// Verifies the replay scene builder produces a non-blank, honest scene from a
// proof receipt: every governed-flow stage becomes a node, the anchor node
// appears only when the proof is anchored, and the assurance never inflates a
// local L3 receipt to L4.

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { bundleToCinemaProof, sceneNodeCount } from '../bundleScene.js';

const localReceipt = {
  status: 'verified',
  assurance: { proofLevel: 'L3', governanceLevel: 'G2', label: 'L3/G2', l0Verified: false, nodeTrusted: false },
  artifacts: { intentId: 'intent-golden-escrow-001', planId: 'plan-x', outcomeId: 'outcome-x', evidenceId: 'bundle-x' },
  subject: { type: 'evidence', id: 'bundle-x' },
};

test('builds a non-blank scene from a receipt', () => {
  const proof = bundleToCinemaProof(localReceipt, null);
  assert.ok(sceneNodeCount(proof) >= 6, 'scene should have the governed-flow nodes');
  assert.ok(Array.isArray(proof.scene.edges) && proof.scene.edges.length >= 5, 'nodes should be linked');
  // edges connect consecutive nodes by id.
  const ids = new Set(proof.scene.nodes.map((n) => n.id));
  for (const e of proof.scene.edges) {
    assert.ok(ids.has(e.from) && ids.has(e.to), 'edge endpoints must be real nodes');
  }
});

test('local L3 receipt never inflates to L4', () => {
  const proof = bundleToCinemaProof(localReceipt, null);
  assert.equal(proof.assurance.l0Verified, false);
  assert.equal(proof.assurance.proofLevel, 'L3');
  assert.equal(proof.assurance.id, 'offline');
  // No anchor node when not anchored.
  assert.ok(!proof.scene.nodes.some((n) => n.id === 'anchor'), 'unanchored proof has no anchor node');
});

test('anchored proof adds the anchor node and L0 assurance', () => {
  const anchored = {
    ...localReceipt,
    assurance: { ...localReceipt.assurance, proofLevel: 'L4', label: 'L4/G2', l0Verified: true },
    artifacts: { ...localReceipt.artifacts, anchorTx: 'deadbeef' },
  };
  const proof = bundleToCinemaProof(anchored, { anchorTxHash: 'deadbeef' });
  assert.ok(proof.scene.nodes.some((n) => n.id === 'anchor'), 'anchored proof shows the anchor node');
  assert.equal(proof.assurance.id, 'l0_confirmed');
  assert.equal(proof.assurance.l0Verified, true);
});

test('handles a missing/empty receipt without throwing', () => {
  const proof = bundleToCinemaProof(undefined, undefined);
  assert.ok(sceneNodeCount(proof) >= 6);
  assert.equal(proof.assurance.l0Verified, false);
});
