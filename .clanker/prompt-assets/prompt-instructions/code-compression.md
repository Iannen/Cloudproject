Target semantics-preserving transformations:
- Symbol Shortening: Reduce symbol names to concise, idiomatic lengths without breaking clarity in narrow scopes.
- Comment Elimination: No comments of any kind are allowed
- Declaration Pruning: Remove intermediate single-use variables, explicit type redundancies, and re-declarations of in-scope values. Use available references instead.

Constraints:
- Do not remove any logging
- Do not remove  otherwise superflous code.