# RED-phase 02 — treasury fixture (§9 attached-method)

Date: 2026-05-15
Fixture: `tests/fixtures/treasury/{widget,manager}/` — two-file scenario representing two distinct realms (`r/example/widget` published by a third party + `r/example/manager` under audit)
Method: two general-purpose subagents, same prompt except one had access to the skill.

## Planted issues

| Location | Class | Severity |
|---|---|---|
| `manager.Promote` calls `w.Tip(cur)` on a `widget.Widget` stored in manager's state | §9 attached-method authority grant | RED |
| `manager.Promote` forwards `cur` to widget's crossing method | §5 cur disclosure | RED |
| `Register` is open (anyone can register, anyone is `Owner`) | composes with §9 — enables drain | RED |
| `Promote` has no payment guard | §1 / §7 | RED |
| `admin` set in `init` but never used | operational (dead) | YELLOW |
| Storing widget.Widget by value, calling `*Widget` method on extracted value | structural smell | YELLOW |

## Results side-by-side

| | Baseline (no skill) | With-skill |
|---|---|---|
| RED findings | 4 | 4 |
| YELLOW findings | 2 | 3 |
| Headline finding | "Widget hijack via unrestricted Register" | "§9 attached-method privilege grant" |
| **§9 named explicitly** | NO ("delegating banker calls to a foreign realm") | YES (cited §9 + Jae's "don't attach objects" doctrine) |
| **§5 cur disclosure caught** | **NO** | **YES** (cited §5) |
| §1 / §7 payment guard caught | partial (F5 YELLOW) | YES (RED, cited §1 + §7) |
| Operational signals (admin dead, overwrite, etc.) | caught individually | caught via the new operational signals table |
| `[uncertain]` markers on Gno specifics | 1 (cross semantics) | 0 |
| Supply-chain framing on widget.New | YES (F4) | implicit in §9 framing |

## The differentiating findings

**§5 cur disclosure** — baseline missed entirely. The subtle bug: `manager.Promote(cur realm, ...)` does `w.Tip(cur)`, passing manager's `cur realm` parameter into a method declared in another realm. Per the spec, `cur` is consumed at the function boundary; forwarding it leaks realm-context authority outward. With-skill caught it as F2; baseline did not flag.

**§9 by name** — baseline caught the SHAPE (third-party banker delegation) but framed it as "design smell" + "supply-chain risk". With-skill named it as the canonical class, cited Jae's doctrine, and gave a structurally-grounded recommendation ("don't store r/ types — use composition with pure values").

**Cross-semantics certainty** — baseline marked `[uncertain]` on whether `w.Tip(cur)` does or doesn't shift realm-context. With-skill cited the exact spec rule from `interrealm.md`.

## Where baseline was better

Baseline's **F4 (supply-chain risk on widget.New)** is a real angle the with-skill agent didn't surface as a standalone finding: a future version of widget could change `Owner` capture without manager noticing. With-skill's §9 framing implicitly covers it ("audit every method as if it were your own code"), but the supply-chain phrasing is more accessible to a non-Gno reviewer.

Worth adding to security.md §9 or patterns.md: "imported `r/` realms are runtime-upgradeable from the dependency author's side — every cross-version audit risk applies."

## Operational signals fired

The new "Operational audit signals" table (added to security.md in Item 1 this round) was used directly by the with-skill agent on F5 (silent overwrite + unchecked type assertion) and F6 (dead admin). The classification "security.md § operational" appeared in two findings — confirming the table is loadable + citable.

## Skill gaps surfaced

The with-skill agent identified one improvement:

> "the auto-addressable pointer-receiver footgun in widget.gno:28 (`*Widget` method on a value extracted from `interface{}`) — it's covered implicitly by §9 but worth a one-liner in patterns.md or security.md § operational."

Specifically: storing a value type, retrieving via type-assertion, then calling a pointer-receiver method auto-addresses against the local copy — mutations don't persist. Subtle Go-vs-Gno gotcha. Not a §9 issue per se but lives near it.

## Actions taken on the skill

1. Added pointer-receiver one-liner to `security.md` § operational (the with-skill agent's identified gap).
2. Added supply-chain caveat to `security.md` §9 (baseline's value-add).

## Verdict

RED-02 was a higher-signal test than RED-01. The skill demonstrably caught at least one bug class (§5) the baseline missed entirely, and resolved the `[uncertain]` markers baseline left around cross semantics. **The skill's specific value on §9-shaped audits is now empirically supported, not just claimed.**

For continued iteration:
- Build a §8 (slice readonly-taint) fixture next — the only `OPEN` bug class in the catalog
- Test against a real `examples/` realm to see how the skill handles noisier code
- Consider whether the `[uncertain]` reduction merits a "load this skill for any Gno work" trigger expansion in the SKILL.md description
