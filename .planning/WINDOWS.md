---
schema_version: 1
open_count: 5
waived_count: 0
fixed_count: 0
total_count: 5
last_updated: 2026-09-10T20:13:17.161Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 01 | deviation | internal/api/service.go | 3722 | Interim window: ContainerMounts renders stored '!' exclusions as stale custom paths in GET /api/containers/{name}/mounts until plan 01-04 Task 1 splits the classes | open |  | 2026-09-09T16:27:39.199Z |  |
| 2 | 01 | unrun-verify | internal/restic/restic_positionals_contract_test.go | 173 | TestPositionalExcludeAbsoluteSubdirPattern has never executed anywhere: skips without restic on PATH and the docker-folders branch is unpushed; CI green Test job (restic 0.17.3) is the arbiter | open |  | 2026-09-10T09:49:14.313Z |  |
| 3 | 3 | deviation | web/src/components/SelectionTree.dom.test.tsx |  | Mock migrated from retired setBackupPaths to setContainerTargets during 03-03 Task 2 (Rule 3); two D-04 copy assertions updated to Task 1 wording | open |  | 2026-09-10T20:13:13.009Z |  |
| 4 | 3 | deviation | web/src/components/SelectionTree.keyboard.dom.test.tsx |  | Mock migrated from retired setBackupPaths to setContainerTargets during 03-03 Task 2 (Rule 3) | open |  | 2026-09-10T20:13:16.924Z |  |
| 5 | 3 | deviation | web/src/pages/Containers.tree.dom.test.tsx |  | RED tests used jest-dom matchers absent from this repo and asserted the shake on the classless presentation wrapper; fixed to plain property checks on the inner row (Rule 1) | open |  | 2026-09-10T20:13:17.161Z |  |

````json
[
  {
    "id": 1,
    "kind": "deviation",
    "phase": "01",
    "file": "internal/api/service.go",
    "line": 3722,
    "description": "Interim window: ContainerMounts renders stored '!' exclusions as stale custom paths in GET /api/containers/{name}/mounts until plan 01-04 Task 1 splits the classes",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-09T16:27:39.199Z",
    "resolved_at": null
  },
  {
    "id": 2,
    "kind": "unrun-verify",
    "phase": "01",
    "file": "internal/restic/restic_positionals_contract_test.go",
    "line": 173,
    "description": "TestPositionalExcludeAbsoluteSubdirPattern has never executed anywhere: skips without restic on PATH and the docker-folders branch is unpushed; CI green Test job (restic 0.17.3) is the arbiter",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T09:49:14.313Z",
    "resolved_at": null
  },
  {
    "id": 3,
    "kind": "deviation",
    "phase": "3",
    "file": "web/src/components/SelectionTree.dom.test.tsx",
    "line": null,
    "description": "Mock migrated from retired setBackupPaths to setContainerTargets during 03-03 Task 2 (Rule 3); two D-04 copy assertions updated to Task 1 wording",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T20:13:13.009Z",
    "resolved_at": null
  },
  {
    "id": 4,
    "kind": "deviation",
    "phase": "3",
    "file": "web/src/components/SelectionTree.keyboard.dom.test.tsx",
    "line": null,
    "description": "Mock migrated from retired setBackupPaths to setContainerTargets during 03-03 Task 2 (Rule 3)",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T20:13:16.924Z",
    "resolved_at": null
  },
  {
    "id": 5,
    "kind": "deviation",
    "phase": "3",
    "file": "web/src/pages/Containers.tree.dom.test.tsx",
    "line": null,
    "description": "RED tests used jest-dom matchers absent from this repo and asserted the shake on the classless presentation wrapper; fixed to plain property checks on the inner row (Rule 1)",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-10T20:13:17.161Z",
    "resolved_at": null
  }
]
````
