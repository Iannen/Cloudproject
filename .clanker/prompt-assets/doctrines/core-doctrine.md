Core packages may only import standard library modules or other core subpackages.
Whitelist: context, errors, fmt, strings, sync, time (types/units/statics only)
- *log :permitted at this stage of development

Keep core logic lean and high-signal: push formatting, data transformation, and multi-step lookups down into adapters.
Core should deal in model objects (or primitives if clearly appropriate). Adapters accept and return domain objects.