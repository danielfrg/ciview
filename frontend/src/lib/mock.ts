// Fictional data for previewing runner and activity UI. Dev server only: opened with ?mock.
import type { Activity, RunnerScope } from "./ci.ts";

const iso = (ms: number) => new Date(ms).toISOString().replace(/\.\d+Z$/, "Z");
const run = (n: number) => `https://github.com/example-org/api/actions/runs/${n}`;

export function mockRunners(now: number): RunnerScope[] {
  const ago = (ms: number) => iso(now - ms);
  const min = 60_000;
  const day = 86_400_000;
  return [
    {
      scope: "org:example-org",
      source: "github",
      state: "healthy",
      kind: "",
      message: "",
      as_of: ago(20_000),
      last_attempt_at: ago(20_000),
      runners: [
        {
          id: "1",
          name: "ops-runner-1",
          os: "Linux",
          status: "online",
          busy: true,
          labels: ["self-hosted", "Linux", "X64", "ops"],
          version: "2.341.0",
          job: {
            title: "example-org/api · Nightly Backup",
            url: run(101),
            started_at: ago(6 * min + 12_000),
          },
        },
        {
          id: "2",
          name: "ops-runner-2",
          os: "Linux",
          status: "offline",
          busy: false,
          labels: ["self-hosted", "Linux", "X64", "backup"],
          version: "2.341.0",
        },
        {
          id: "3",
          name: "mac-ci-1",
          os: "macOS",
          status: "online",
          busy: false,
          labels: ["self-hosted", "macOS", "ARM64", "ios"],
          version: "2.338.2",
          update: { latest: "2.341.0", missing_since: ago(12 * day), overdue: false },
        },
        {
          id: "4",
          name: "build-box-3",
          os: "Linux",
          status: "online",
          busy: true,
          labels: ["self-hosted", "Linux", "X64", "gpu"],
          version: "2.329.0",
          update: { latest: "2.341.0", missing_since: ago(41 * day), overdue: true },
          job: { title: "example-org/ml · Train Model", url: run(102), started_at: ago(64 * min) },
        },
      ],
    },
    {
      scope: "repo:example-owner/homelab",
      source: "github",
      state: "healthy",
      kind: "",
      message: "",
      as_of: ago(20_000),
      last_attempt_at: ago(20_000),
      runners: [
        {
          id: "5",
          name: "homelab-runner",
          os: "Linux",
          status: "online",
          busy: true,
          labels: ["self-hosted", "Linux", "X64", "ops-home"],
          version: "2.341.0",
          job: {
            title: "example-owner/homelab · Archive",
            url: run(103),
            started_at: ago(12 * min),
          },
        },
      ],
    },
    {
      scope: "org:example-labs",
      source: "github",
      state: "stale",
      kind: "rate_limit",
      message: "Provider rate limit reached",
      as_of: ago(8 * min),
      last_attempt_at: ago(30_000),
      runners: [
        {
          id: "6",
          name: "lab-runner",
          os: "Linux",
          status: "online",
          busy: false,
          labels: ["self-hosted", "Linux", "X64"],
          version: "2.341.0",
        },
      ],
    },
  ];
}

export function mockActivity(now: number): Activity {
  const ago = (ms: number) => iso(now - ms);
  const min = 60_000;
  return {
    running: [
      {
        key: "r1",
        repository: "example-org/api",
        workflow: "Nightly Backup",
        branch: "main",
        url: run(101),
        since: ago(6 * min + 12_000),
        pool: "self-hosted",
        runner: "ops-runner-1",
        hint: null,
      },
      {
        key: "r2",
        repository: "example-org/ml",
        workflow: "Train Model",
        branch: "main",
        url: run(102),
        since: ago(64 * min),
        pool: "self-hosted",
        runner: "build-box-3",
        hint: null,
      },
      {
        key: "r3",
        repository: "example-owner/homelab",
        workflow: "Archive",
        branch: "main",
        url: run(103),
        since: ago(12 * min),
        pool: "self-hosted",
        runner: "homelab-runner",
        hint: null,
      },
      {
        key: "r4",
        repository: "example-org/web",
        workflow: "CI",
        branch: "main",
        url: run(104),
        since: ago(100_000),
        pool: "github-hosted",
        runner: null,
        hint: null,
      },
    ],
    queued: [
      {
        key: "q1",
        repository: "example-org/web",
        workflow: "Nightly Backup",
        branch: "main",
        url: run(105),
        since: ago(14 * min),
        pool: "self-hosted",
        runner: null,
        hint: { tone: "warning", text: "No online runner with label backup" },
      },
      {
        key: "q2",
        repository: "example-org/api",
        workflow: "Deploy",
        branch: "main",
        url: run(106),
        since: ago(3 * min + 20_000),
        pool: "self-hosted",
        runner: null,
        hint: { tone: "info", text: "Waiting for ops-runner-1 (busy)" },
      },
      {
        key: "q3",
        repository: "example-owner/homelab",
        workflow: "Nightly Sync",
        branch: "main",
        url: run(107),
        since: ago(2 * min),
        pool: "self-hosted",
        runner: null,
        hint: { tone: "info", text: "Waiting for homelab-runner (busy)" },
      },
      {
        key: "q4",
        repository: "example-org/web",
        workflow: "Build Image",
        branch: "main",
        url: run(108),
        since: ago(8_000),
        pool: "github-hosted",
        runner: null,
        hint: null,
      },
    ],
  };
}
