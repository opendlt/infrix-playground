# receipts/

Runtime directory for share-linked proof receipts.

When the server is started with `--receipt-dir hosted-playground/receipts`, each
completed run is persisted here as `<id>.run.json` (the public proof receipt +
portable bundle — no private keys, no sensitive data) so share links survive a
restart. These files are runtime data and are gitignored; the daily cleanup job
prunes them past the retention window.

In the default configuration receipts are kept in memory only and nothing is
written here.
