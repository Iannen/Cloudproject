Core packages may only import standard library modules or other core subpackages.
Whitelisted standard library imports (* indicates that the import is targeted for core elimination):
- context
- errors
- fmt
- strings
- sync
- *os
- *encoding/json
- *log
- *time

Keep core logic lean and high-signal: push formatting, data transformation, and multi-step lookups down into adapters.
Core should deal in model objects (or primitives if clearly appropriate). Adapters accept and return domain objects.