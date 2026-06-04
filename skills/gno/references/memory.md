# Memory and persistence

> **Category: VM model.** Update when the VM's persistence model, heap-item semantics, or typed-value layout changes.
> **Authoritative spec**: `docs/resources/gno-memory-model.md` and `docs/resources/gno-data-structures.md` in the `gnolang/gno` repo. This reference is the practical author/auditor-oriented summary; load the spec when an audit hinges on exact semantics.

## Why this reference exists

The persistence model determines what survives across transactions, what carries identity (and therefore authority), and where the gas/storage costs accrue. Several bug classes (`security.md` Class 4 closed-over-authority, the (B)-class no-anchor laundering vector, the (A) `/r/`-declared-types rule) make sense only against this model. Read this before reasoning about state shape, closure capture, or whether a value persists.

## The typed value

Every value in Gno is stored as a `TypedValue` tuple:

```go
type TypedValue struct {
    T Type     // the static type (interface value)
    V Value    // the dynamic value (interface value)
    N [8]byte  // primitive payload (bools, ints) — perf optimization
}
```

`T` and `V` are both Go interface values. Primitive values like `bool` and `int` are stored in `N` for performance. Vars in scope blocks, struct fields, array elements, map keys/values — all are `TypedValue` slots.

This tuple representation is uniform: reading and writing values has the same code path regardless of whether the slot holds an interface or a concrete type. There are no memory-layout optimizations for "small" types beyond the `N` field.

## Objects vs values

There are many value types. A subset are also **objects** — they carry identity (`ObjectInfo`) and a `PkgID` stamp. Objects can be persisted; non-object values are inlined wherever they appear.

| Value type | Object? | Notes |
|---|---|---|
| Primitive (bool, ints, floats) | no | inlined; stored in `N` |
| `StringValue` | no | inlined |
| `BigintValue`, `BigdecValue` | no | only used for constant expressions |
| `DataByteValue` | no | invisible byte-array optimization |
| `PointerValue` | no | the *base* it points to is always an object |
| **`ArrayValue`** | **yes** | |
| `SliceValue` | no | references an `ArrayValue` underlying |
| **`StructValue`** | **yes** | most user-defined structs |
| **`FuncValue`** | **yes** | closures and methods |
| **`MapValue`** | **yes** | |
| **`BoundMethodValue`** | **yes** | func + receiver |
| `TypeValue`, `PackageValue` | no | |
| **`BlockValue`** | **yes** | for package, file, if, range, switch, func bodies |
| `RefValue` | no | reference to an object stored on disk (lazy load) |
| **`HeapItemValue`** | **yes** | invisible type for loopvars and closure captures |

**Why this matters**: objects get `ObjectID.PkgID` stamped at allocation time — that's `m.Realm.ID`, the realm-storage-context. Identity = authority. See `interrealm.md` § Storage = Authority.

## Real vs unreal

Every object Gno allocates is either:

- **Unreal** — allocated this transaction. May or may not become real at finalization.
- **Real** — has a finalized `ObjectID.NewTime` and persists on disk.

`ObjectID` has three states:

| State | `PkgID` | `NewTime` | Meaning |
|---|---|---|---|
| empty | zero | zero | Never went through the allocator |
| allocated | set | zero | In memory; authority known, not persisted |
| finalized | set | non-zero | Real; persisted with a tx-stamped `NewTime` |

At realm-transaction finalization (`interrealm.md` § Realm boundaries and finalization):

- Unreal objects reachable from persisted state become real.
- Unreachable objects (refcount zero) are garbage-collected.
- Modified objects' Merkle hashes are recomputed.

**Practical consequence**: an object you allocate and then drop on the floor (no reachable reference at boundary exit) is GC'd at finalization — no gas owed for storage, no persistence. An object you allocate and link into persisted state survives, locks storage deposit (`patterns.md` § Cost-aware design), and gets a stable `NewTime`.

## Pointers

```go
type PointerValue struct {
    TV    *TypedValue  // the typed-value slot the pointer references
    Base  Value        // array / struct / block, or heap item
    Index int          // list/fields/values index, or -1/-2 for special cases
}
```

The `Base` is always an object — that's how the realm finalizer knows what to update when a pointer is written through. The pointer doesn't carry its own identity; identity lives in `Base`.

**Why this matters for authority**: a pointer to a real foreign-stamped value carries the readonly taint. The taint is sticky and propagates through `.Field`, `[i]`, slicing, and copies (`interrealm.md` § Readonly taint). The pointer itself is a reference; the borrow rules fire on the underlying object's PkgID.

## Blocks and heap items

Every `{...}` block in Gno produces a `BlockValue`:

```go
type BlockValue struct {
    ObjectInfo
    Source   BlockNode
    Values   []TypedValue   // one slot per declared var
    Parent   Value
    Blank    TypedValue     // captures "_" underscore names
    bodyStmt bodyStmt
}
```

The AST nodes that create a new block on execution: `FuncLitStmt`, `BlockStmt`, `ForStmt`, `IfCaseStmt`, `RangeStmt`, `SwitchCaseStmt`, `FuncDecl`, `FileNode`, `PackageNode`.

**Heap items** are an invisible object type that wrap a single typed-value slot:

```go
&HeapItemValue{TypedValue{T, V, N}}
```

The Gno preprocessor identifies variables that need a heap item — those that are:

1. Referenced by a pointer that escapes the block (`return &x`).
2. Captured by a closure (`func() { ... uses x ... }`).
3. Declared at package (global) level.
4. Loop variables in `for ... := range` whose iteration value is captured by a closure or pointer.

When the preprocessor marks a slot, `NewBlock()` prepopulates that slot with `*HeapItemValue` instead of a zero typed-value. Writes to the variable go *through* the heap item; reads dereference it.

### Why heap items exist — closure independence

```go
func Example2(arg int) (res func()) {
    var x int = arg + 1
    return func() {
        println(x)
    }
}
```

Without heap items, the closure would have to hold a reference to the *whole* enclosing block, defeating per-tx GC. With heap items, the closure captures only the single heap item — when invoked in a later transaction, the VM loads only that one object, not the original block.

This is what makes closures truly first-class persisted values: they survive across transactions independently of the frame that created them.

### Loop variables — Go 1.22 semantics

Gno implements Go 1.22's loopvar fix using heap items:

```go
for _, v := range values {
    saveClosure(func() {
        println(v)
    })
}
```

Instead of all closures capturing the same `v` slot (the pre-1.22 footgun), the VM **replaces the heap item with a new one** at each iteration. Each closure captures a distinct heap item with the iteration's value frozen.

This is called a **heap definition**. The preprocessor identifies loopvars and goto-implicit-loop vars that get captured or pointer-referenced, and directs the VM to perform this redefinition each iteration.

### Globals are heap items

Package-level `var` declarations are wrapped in heap items. This serves two purposes:

1. **Closure independence**: a global captured by a closure becomes a single heap-item reference, not a back-pointer to the whole package block.
2. **Future upgrade hook**: the heap-item indirection allows the VM to eventually support adding new functions or swizzling existing ones in mutable realm packages.

For now, this means: every package-level `var` allocates its own heap item at deploy time. Each is an object with a PkgID stamp. Each pays storage deposit.

## Practical implications

### Returning `&x` triggers heap allocation

If a function returns a pointer to a local variable, the preprocessor marks that var for a heap item. The variable lives on the heap (an object), not the stack — and survives the function return because the pointer keeps it reachable.

This is what makes the safe-object pattern (`patterns.md`) work: a constructor that does `return &state{...}` returns a pointer to a real heap-item-wrapped struct. The struct outlives the constructor's frame.

### Storing closures across transactions works

Because a closure captures heap items (not block contents), storing a `func()` in realm state is well-defined. The captured environment is the set of heap items referenced. As long as those heap items remain reachable, the closure can be invoked in a later transaction.

This is also what makes Class 4 (closed-over-authority) bugs possible: a stored closure carries the **creator-realm's authority** (D3 closure-capability borrow) and the **captured heap items** independently — exactly the trust-token shape that's exploitable. See `security.md` Class 4.

### Slice descriptors cross-realm

Slices hold references to an underlying backing array. The slice header itself isn't an object — but the backing array IS. Reading a slice from another realm gives you a header pointing at the foreign realm's array, taint-marked. Element mutation panics.

To work with a foreign slice locally, **clone on entry**:

```go
local := append([]int(nil), foreign...)
```

This copies values into a new array allocated under the current realm's `m.Realm`.

### Map iteration order — Gno vs Go

Gno's `map` iterates in **insertion order**, not Go's randomized order. This makes the in-memory iteration deterministic. **However**:

1. Iteration order is still an implementation detail. Don't write code that depends on it.
2. Maps are not persistence-friendly — every map mutation reloads and rewrites the whole map's persisted form. Use `avl.Tree` or `bptree` for any keyed state that grows.
3. The deterministic iteration order is *not* portable to vanilla Go — code that runs under both must not rely on it.

See `patterns.md` § State shape for the AVL-over-map rule.

## Data structure cheat-sheet

| Type | Best for | Persistence behavior |
|---|---|---|
| `[N]T` (array) | fixed-size collections | full slot persisted; each element a slot |
| `[]T` (slice) | dynamic lists | backing array is the object; descriptor is value |
| `map[K]V` | small bounded in-memory state, config values | persisted as a single object — whole map rewrites on mutation |
| `avl.Tree` (`p/nt/avl/v0`) | growing keyed state, sorted iteration | lazy load — only the search path touches storage |
| `bptree` (`p/nt/bptree/v0`) | growing keyed state, range queries | lazy load like AVL |
| `struct{...}` | grouped data | one object per real instance; fields are slots |
| `*T` | reference passing, mutation through pointers | the base it points to is the object |

### When to use what

- **Persistent growth state**: AVL (or B+tree). Lazy load + deterministic iteration + clean refcount.
- **Small in-memory config or transient computation**: Go map is fine.
- **Fixed-size known-at-compile-time data**: array.
- **Lists that grow but stay bounded and you iterate fully**: slice.
- **Multi-index access**: multiple AVLs (`patterns.md` § Index over scan example).

## What does NOT persist

- Local variables in a function whose pointer doesn't escape.
- Unreal objects unreachable from persisted state at boundary exit.
- The `cur realm` capability token — explicitly refused at attachment time. Capture `cur.Address()` / `cur.PkgPath()` (strings) if you need them across transactions. See `interrealm.md` § Realm values are ephemeral.
- `revive()` frame state (it's a test-only builtin currently).

## Cross-references

- `interrealm.md` — Storage = Authority (PkgID stamping), borrow rules that fire based on object PkgID, readonly taint mechanics, conversion guards
- `security.md` — Class 4 closed-over-authority (closures and heap items as trust tokens), the (A)/(B)/(C) safety hypothesis (which relies on object identity)
- `patterns.md` — state shape decisions (AVL over map, single-struct upgradability, cost-aware design)
- `stdlib.md` — `gno.land/p/nt/avl/v0` and `p/nt/bptree/v0` API

## Source

- `docs/resources/gno-memory-model.md` — typed value, blocks, heap items, loopvars.
- `docs/resources/gno-data-structures.md` — arrays, slices, maps, structs, AVL.
- `gnovm/pkg/gnolang/alloc.go` and `realm.go` — implementation: allocation, finalization, refcounting.
