# Behavior retained during normalization

These follow-ups identify behavior that requires a separate decision and focused
fix. They are exceptions to the desired long-term conventions, preserved so the
normalization can be reviewed as a structural change.

| ID | Retained behavior | Follow-up acceptance |
| --- | --- | --- |
| NORM-001 | During normalization, profile updates included a balance observed before the write, potentially overwriting a concurrent coin change. | Profile writes use an explicit column set excluding balance. An integration regression interleaves a committed balance deduction before the profile patch and verifies the new balance is retained. |
| NORM-002 | Slot weights and map iteration determine outcomes, with the existing gold fallback. Ordering or normalizing the weights would change gameplay. | Agree the probability model and verify each balance tier and fallback distribution. |
| NORM-003 | Slot reward multiplication historically discards a checked arithmetic error. Its handling remains a documented, local lint exception. | Decide overflow response/rollback behavior and add a business regression test before changing it. |
| NORM-004 | Server shutdown created a timeout without applying it to Fiber shutdown. | HTTP lifecycle resolved: pass the 10-second context to Fiber and verify in-flight requests complete before the deadline or time out when it expires. Closing PostgreSQL and Redis clients is separate lifecycle work. |
| NORM-005 | Existing configuration/database startup helpers may panic or exit; unknown logger environments may return nil. | Move recoverable initialization failures to explicit error returns and define executable exit behavior. |
| NORM-006 | Match list retrieval retains its per-item lookups and bet-count queries. | Batch loading while preserving result ordering and current odds calculations. |

The preferred self-profile endpoint is now `PATCH /api/v1/users/me`. It derives
the account ID from authentication and accepts only name, nickname, and group
changes. Omitted fields remain unchanged; explicit null clears nickname or group.
The legacy `PATCH /api/v1/users/:id` endpoint is deprecated and returns 403 when
the path ID differs from the signed-in user's ID. For matching IDs it delegates
to the same self-profile handler, request DTO, service, and repository operation.
The old profile update DTO, input, service method, and repository method have
been removed.
PostgreSQL-backed HTTP integration tests cover the new endpoint with omitted and
explicitly null fields, matching-ID legacy delegation, mismatched-ID rejection,
and rejection of balance writes. The tests inject the authenticated actor at the
route middleware boundary.

The Go module, CI checks, linter, and Docker builder target Go 1.27.1.
