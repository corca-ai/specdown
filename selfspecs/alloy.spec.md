# Alloy Models

Alloy fragments can be embedded directly in a spec document using
`alloy:model(name)` code blocks.

Fragments with the same model name are combined in document order
into a single logical model. Only the first fragment may contain
a `module` declaration.

To link an assertion check result to the current section:

```
<!-- alloy:ref(modelName#assertionName, scope=5) -->
```

This directive displays the check result as a status badge in the HTML report.

## Formal Properties

The document model has a simple structural invariant:
every executable block belongs to exactly one heading scope.

```alloy:model(docmodel)
module docmodel

sig Heading {}

sig Block {
  scope: one Heading
}

sig TableRow {
  scope: one Heading
}
```

A block must not belong to more than one heading scope.

```alloy:model(docmodel)
assert blockBelongsToOneScope {
  all b: Block | one b.scope
}

check blockBelongsToOneScope for 5
```

<!-- alloy:ref(docmodel#blockBelongsToOneScope, scope=5) -->

A table row must not belong to more than one heading scope.

```alloy:model(docmodel)
assert rowBelongsToOneScope {
  all r: TableRow | one r.scope
}

check rowBelongsToOneScope for 5
```

<!-- alloy:ref(docmodel#rowBelongsToOneScope, scope=5) -->
