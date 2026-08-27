Core packages may only import standard library modules or other core subpackages.
Whitelist: context, errors, fmt, strings, sync, time (types/units/statics only)
- *log :permitted at this stage of development

Core logic -> lean and high-signal.
Formatting, data transformation and multi-step lookups -> down into adapters.
Core deals in model objects/primitives(if clearly appropriate) -> adapters accept and return core models members.
Registry and RPCHandler are allowed to carry appctx, for purpose of long lived role-goroutine LC management