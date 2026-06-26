// Infrix Playground — the Tamper Lab forgery engine (RB-02).
//
// Each forgery mutates a CLONE of a PortableEvidencePackage so a visitor can try
// to defeat the proof and watch the maths catch them. Two tiers, by design:
//
//   • Naive  — change a field and stop. The ExportHash is the outer seal over
//     almost the whole package, so ANY untouched-elsewhere edit is caught
//     immediately at check #2 (export_hash). One forgery demonstrates this.
//
//   • Sophisticated — a real forger RE-SEALS the ExportHash after editing, so it
//     sails past check #2. The package is then caught by the INNER cryptographic
//     binding instead (plan↔approval, the Merkle inclusion proof, the policy
//     digest, the trust snapshot, plugin provenance…). Each of these forgeries
//     trips one specific inner check.
//
// Why injection: re-sealing must recompute the ExportHash byte-for-byte the way
// the verifier does. Rather than re-implement (and risk diverging from) the
// canonical JSON encoder, this module takes the shared canonical hasher as an
// injected dependency (`makeReseal`). The browser passes the real one from
// /lib/canonicalJson.js — the SAME primitive the verifier uses — so a re-sealed
// package genuinely passes export_hash. This also keeps tamper.js free of any
// /lib import, so it loads and unit-tests in plain Node (no module-cache needed).
//
// Served as a top-level playground asset at /tamper.js (embedded in web.go); the
// /lib and /components URL prefixes are owned by the shared Nexus handler.

const clone = (pkg) => JSON.parse(JSON.stringify(pkg));

// Parse/mutate/restore bundleData whether it's an inlined object or a JSON string
// (json.RawMessage can serialize either way).
function withBundle(pkg, fn) {
  let bd = pkg.bundleData;
  const wasString = typeof bd === 'string';
  if (wasString) bd = JSON.parse(bd);
  fn(bd);
  pkg.bundleData = wasString ? JSON.stringify(bd) : bd;
  return pkg;
}

function approvals(bd) {
  return bd.approvalEvidence || bd.ApprovalEvidence || [];
}

// flipFirst returns a copy of a [N]byte array with byte 0 inverted — a minimal,
// deterministic edit guaranteed to change the value.
function flipFirst(arr) {
  if (!Array.isArray(arr) || arr.length === 0) return arr;
  return arr.map((b, i) => (i === 0 ? (b ^ 0xff) & 0xff : b));
}

// FORGERIES — ordered for the lab: the naive outer-seal demo first, then one per
// inner binding. `trips` is the check the verifier ACTUALLY fails on (verified by
// the E2E harness and pinned in tamper.test.mjs). `reseal` forgeries require the
// injected re-seal fn so they pass export_hash and reach their inner check.
export const FORGERIES = [
  {
    id: 'approver',
    label: 'Rename the approver',
    blurb: 'Swap the approving identity — without re-sealing. The export hash catches it instantly.',
    trips: 'export_hash',
    reseal: false,
    mutate: (p) => withBundle(p, (bd) => {
      const a = approvals(bd)[0];
      if (a) a.identity = 'acc://attacker.evil/admin';
    }),
  },
  {
    id: 'planhash',
    label: 'Break plan ↔ approval',
    blurb: 'Re-seal the export hash, but point the approval at a different plan. The binding still catches it.',
    trips: 'plan_hash',
    reseal: true,
    mutate: (p) => withBundle(p, (bd) => {
      const a = approvals(bd)[0];
      if (a && Array.isArray(a.planHash)) a.planHash = flipFirst(a.planHash);
    }),
  },
  {
    id: 'outcome',
    label: 'Rewrite the outcome',
    blurb: 'Re-seal, but change the committed outcome digest. It no longer matches the bundle.',
    trips: 'outcome_digest',
    reseal: true,
    mutate: (p) => { if (Array.isArray(p.outcomeDigest)) p.outcomeDigest = flipFirst(p.outcomeDigest); return p; },
  },
  {
    id: 'chainhash',
    label: 'Swap a chain hash',
    blurb: 'Re-seal, but tamper with one Merkle inclusion proof. The root no longer reconstructs.',
    trips: 'inclusion_proof[0]',
    reseal: true,
    mutate: (p) => {
      const pr = (p.inclusionProofs || [])[0];
      if (pr && pr.link && Array.isArray(pr.link.contentHash)) pr.link.contentHash = flipFirst(pr.link.contentHash);
      return p;
    },
  },
  {
    id: 'policy',
    label: 'Drop a policy decision',
    blurb: 'Re-seal, but delete a recorded policy decision. The policy digest no longer matches.',
    trips: 'policy_decision_digest',
    reseal: true,
    mutate: (p) => withBundle(p, (bd) => {
      if (Array.isArray(bd.policyDecisions) && bd.policyDecisions.length) bd.policyDecisions = bd.policyDecisions.slice(1);
    }),
  },
  {
    id: 'trust',
    label: 'Forge a trust snapshot',
    blurb: 'Re-seal, but zero a trust snapshot’s block height. A snapshot must be captured at a real block.',
    trips: 'trust_snapshot[0]',
    reseal: true,
    mutate: (p) => { const s = (p.trustSnapshot || [])[0]; if (s) s.blockHeight = 0; return p; },
  },
  {
    id: 'plugin',
    label: 'Hide a plugin version',
    blurb: 'Re-seal, but blank a plugin’s implementation hash. Every plugin must be fully identified.',
    trips: 'plugin_versions[0]',
    reseal: true,
    mutate: (p) => { const pv = (p.pluginVersions || [])[0]; if (pv) pv.implementationHash = ''; return p; },
  },
];

/**
 * applyForgery clones `pkg`, applies the named forgery, and (for re-seal
 * forgeries) recomputes the ExportHash with the injected `reseal` fn.
 * @param {object} pkg     the clean package (never mutated)
 * @param {string} id      a FORGERIES id
 * @param {(p:object)=>Promise<object>} [reseal]  required iff the forgery re-seals
 * @returns {Promise<{pkg:object, forgery:object}>}
 */
export async function applyForgery(pkg, id, reseal) {
  const forgery = FORGERIES.find((f) => f.id === id);
  if (!forgery) throw new Error('unknown forgery ' + id);
  const p = forgery.mutate(clone(pkg));
  if (forgery.reseal) {
    if (typeof reseal !== 'function') throw new Error(`forgery "${id}" requires a reseal function`);
    await reseal(p);
  }
  return { pkg: p, forgery };
}

/**
 * makeReseal builds the re-seal function from the shared canonical hasher. It
 * recomputes the ExportHash exactly as pkg/evidence.computePortableExportHash
 * (mirrored in /lib/portableVerifier.js::computeExportHash) does, so the verifier
 * accepts the result. `canon` must provide `canonicalJSONSha256` and `coerce32`
 * from /lib/canonicalJson.js.
 */
export function makeReseal(canon) {
  const arrayifyHash = (v) => Array.from(canon.coerce32(v));
  const bundleDataAsCanonicalValue = (bd) => {
    if (bd === null || bd === undefined) return null;
    if (typeof bd === 'string') { try { return JSON.parse(bd); } catch { return bd; } }
    return bd;
  };
  return async function reseal(pkg) {
    const intermediate = {
      version: pkg.version,
      bundleData: bundleDataAsCanonicalValue(pkg.bundleData),
      planHash: arrayifyHash(pkg.planHash),
      outcomeDigest: arrayifyHash(pkg.outcomeDigest),
      trustSnapshot: pkg.trustSnapshot || null,
      inclusionProofs: pkg.inclusionProofs || null,
      anchorProof: pkg.anchorProof || null,
      anchorTxHash: pkg.anchorTxHash || '',
      anchorBlockHeight: Number(pkg.anchorBlockHeight || 0),
      pluginVersions: pkg.pluginVersions || null,
      policyDecisionDigest: arrayifyHash(pkg.policyDecisionDigest),
      replayCapsule: bundleDataAsCanonicalValue(pkg.replayCapsule),
    };
    const h = await canon.canonicalJSONSha256(intermediate);
    pkg.exportHash = Array.from(h);
    return pkg;
  };
}
