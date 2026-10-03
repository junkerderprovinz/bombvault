// The placement every staged item carries: stored in the domain's own
// repository and copied to every target, both following the domain's default.
// The server sends one with each container, VM and folder set.
export const PLACEMENT = {
  segment: "local-offsite",
  repo: "",
  repoLabel: "",
  repoKind: "domain",
  repoOff: false,
  homeFollows: true,
  copiesFollow: true,
  skip: [],
  locked: false,
  lockReason: "",
  segmentLocks: {},
  paused: false,
  unreadable: false,
};
