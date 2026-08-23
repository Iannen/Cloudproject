Implement & refine a 'Core vs. Adapters' Architecture:
- core/: Houses high-level orchestration, state machine transitions, and decision logic, remaining strictly free of low-level driver and protocol dependencies.
- adapters/: Houses low-level execution mechanics, including storage drivers, SDK integrations, network transports, and environment configuration.
    - is allowed to import models, so it can receive and return model members to core.
- main/: bootstrap, DI and the registry (which intentionally holds adapter structs not itfs)