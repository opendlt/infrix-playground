// RB-02 — Tamper Lab forgery-engine smokes.
//
// These pin the MUTATIONS and the engine contract (clone safety, re-seal
// injection, the forgery set). What each forgery actually trips in the verifier
// is proven separately by the real-verifier E2E (see the runbook's verification
// log) — that needs the nexus-web module cache, which the web CI job doesn't
// have, so this file imports only tamper.js and stays dependency-free.

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { FORGERIES, applyForgery, makeReseal } from '../tamper.js';

function makePkg() {
  return {
    version: '4',
    bundleData: {
      id: 'ev-test',
      approvalEvidence: [{ identity: 'acc://honest.acme/admin', role: 'approver', planHash: [9, 9, 9] }],
      policyDecisions: [{ decision: 'allow', ruleId: 'must-allow' }, { decision: 'allow', ruleId: 'extra' }],
      outcomeDigest: [1, 2, 3],
    },
    planHash: [9, 9, 9],
    outcomeDigest: [1, 2, 3],
    policyDecisionDigest: [4, 5, 6],
    inclusionProofs: [{ link: { contentHash: [7, 7, 7] }, proof: [] }],
    trustSnapshot: [{ profileId: 'p', blockHeight: 987 }],
    pluginVersions: [{ pluginId: 'pkg.demo', version: '1.0.0', implementationHash: 'abc' }],
    exportHash: [0, 0, 0],
  };
}

// A fake re-seal so this test never needs the real canonical hasher.
const fakeReseal = async (p) => { p.exportHash = ['SEALED']; return p; };

test('the forgery set is exactly the seven expected, with pinned trips', () => {
  assert.deepEqual(FORGERIES.map((f) => f.id),
    ['approver', 'planhash', 'outcome', 'chainhash', 'policy', 'trust', 'plugin']);
  const trips = Object.fromEntries(FORGERIES.map((f) => [f.id, f.trips]));
  assert.deepEqual(trips, {
    approver: 'export_hash',
    planhash: 'plan_hash',
    outcome: 'outcome_digest',
    chainhash: 'inclusion_proof[0]',
    policy: 'policy_decision_digest',
    trust: 'trust_snapshot[0]',
    plugin: 'plugin_versions[0]',
  });
  // Only the naive one skips the re-seal; the rest must re-seal.
  assert.equal(FORGERIES.find((f) => f.id === 'approver').reseal, false);
  assert.ok(FORGERIES.filter((f) => f.id !== 'approver').every((f) => f.reseal === true));
});

test('applyForgery never mutates the clean package', async () => {
  const clean = makePkg();
  const snapshot = JSON.stringify(clean);
  for (const f of FORGERIES) {
    await applyForgery(clean, f.id, fakeReseal);
    assert.equal(JSON.stringify(clean), snapshot, `forgery ${f.id} leaked a mutation into the clean pkg`);
  }
});

test('approver is naive (no re-seal) and rewrites the identity', async () => {
  const { pkg, forgery } = await applyForgery(makePkg(), 'approver');
  assert.equal(forgery.reseal, false);
  assert.equal(pkg.bundleData.approvalEvidence[0].identity, 'acc://attacker.evil/admin');
  assert.deepEqual(pkg.exportHash, [0, 0, 0], 'naive forgery must NOT re-seal');
});

test('re-seal forgeries require a re-seal fn and apply it', async () => {
  await assert.rejects(() => applyForgery(makePkg(), 'planhash'), /requires a reseal/);
  const { pkg } = await applyForgery(makePkg(), 'planhash', fakeReseal);
  assert.deepEqual(pkg.exportHash, ['SEALED'], 're-seal fn must run');
  assert.notDeepEqual(pkg.bundleData.approvalEvidence[0].planHash, [9, 9, 9], 'approval planHash flipped');
});

test('each forgery makes its targeted edit', async () => {
  const outcome = (await applyForgery(makePkg(), 'outcome', fakeReseal)).pkg;
  assert.notEqual(outcome.outcomeDigest[0], 1, 'outcome digest byte flipped');

  const chain = (await applyForgery(makePkg(), 'chainhash', fakeReseal)).pkg;
  assert.notEqual(chain.inclusionProofs[0].link.contentHash[0], 7, 'chain content hash flipped');

  const policy = (await applyForgery(makePkg(), 'policy', fakeReseal)).pkg;
  assert.equal(policy.bundleData.policyDecisions.length, 1, 'one policy decision dropped');

  const trust = (await applyForgery(makePkg(), 'trust', fakeReseal)).pkg;
  assert.equal(trust.trustSnapshot[0].blockHeight, 0, 'trust snapshot block height zeroed');

  const plugin = (await applyForgery(makePkg(), 'plugin', fakeReseal)).pkg;
  assert.equal(plugin.pluginVersions[0].implementationHash, '', 'plugin implementation hash blanked');
});

test('applyForgery rejects an unknown forgery id', async () => {
  await assert.rejects(() => applyForgery(makePkg(), 'nope', fakeReseal), /unknown forgery/);
});

test('makeReseal handles a string bundleData and sets a 32-byte exportHash', async () => {
  // Inject a deterministic stand-in for the canonical hasher.
  const canon = {
    coerce32: (v) => new Uint8Array(Array.isArray(v) ? v.slice(0, 32) : []),
    canonicalJSONSha256: async () => new Uint8Array(32).fill(7),
  };
  const reseal = makeReseal(canon);
  const pkg = makePkg();
  pkg.bundleData = JSON.stringify(pkg.bundleData); // RawMessage-as-string form
  await reseal(pkg);
  assert.equal(pkg.exportHash.length, 32);
  assert.ok(pkg.exportHash.every((b) => b === 7));
});
