# Behavior retained during normalization

These follow-ups identify behavior that requires a separate decision and focused
fix. They are exceptions to the desired long-term conventions, preserved so the
normalization can be reviewed as a structural change.

| ID | Retained behavior | Follow-up acceptance |
| --- | --- | --- |
| NORM-001 | During normalization, profile updates included a balance observed before the write, potentially overwriting a concurrent coin change. | Profile writes now use an explicit column set excluding balance. Concurrent balance preservation still needs regression verification. |
| NORM-002 | Slot weights and map iteration determine outcomes, with the existing gold fallback. Ordering or normalizing the weights would change gameplay. | Agree the probability model and verify each balance tier and fallback distribution. |
| NORM-003 | Slot reward multiplication historically discards a checked arithmetic error. Its handling remains a documented, local lint exception. | Decide overflow response/rollback behavior and add a business regression test before changing it. |
| NORM-004 | Server shutdown creates a timeout but does not apply it to Fiber shutdown. | Specify bounded shutdown and verify outstanding requests and resources finish correctly. |
| NORM-005 | Existing configuration/database startup helpers may panic or exit; unknown logger environments may return nil. | Move recoverable initialization failures to explicit error returns and define executable exit behavior. |
| NORM-006 | Match list retrieval retains its per-item lookups and bet-count queries. | Batch loading while preserving result ordering and current odds calculations. |

The public user update endpoint continues to operate on the authenticated actor,
including when its path contains another ID. Its observed semantics and profile
timestamps are protected by regression tests; redefining authorization or path
semantics requires a separate contract change.

The Go module and Docker compiler remain at their existing declarations. A
supported toolchain upgrade should update those declarations together with CI.
