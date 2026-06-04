# agent_tools
A agent tools framework in go suppling tools ecosystem for agents written in go


default structure:

```
agenttools/
├── go.mod
├── README.md
│
├── tool/                  # Core interfaces and types
│   ├── tool.go
│   ├── result.go
│   └── schema.go
│
├── registry/              # Tool registration/discovery
│   └── registry.go
│
├── tools/
│   ├── search/
│   │   ├── search.go
│   │   └── provider.go
│   │
│   ├── remember/
│   │   ├── remember.go
│   │   └── store.go
│   │
│   ├── cron/
│   │   ├── cron.go
│   │   └── scheduler.go
│   │
│   ├── shell/
│   │   └── shell.go
│   │
│   └── filesystem/
│       └── filesystem.go
│
└── examples/
    └── basic/
```






