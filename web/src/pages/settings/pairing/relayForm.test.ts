// The server stores one mode for an own relay and for a paired BombVault that
// serves as one. The card tells them apart by what the stored state says.
import { describe, expect, it } from "vitest";

import { relayForm } from "./relayForm";

const members = [{ address: "https://192.168.1.21:3443" }, { address: "" }];
const relay = (over: Partial<Parameters<typeof relayForm>[0]>) => ({ mode: "own" as const, url: "", serve: false, ...over });

describe("which route the stored relay is", () => {
  it("is the stored mode for the project relay and for none", () => {
    expect(relayForm(relay({ mode: "project", url: "wss://192.168.1.21/relay/connect" }), members)).toBe("project");
    expect(relayForm(relay({ mode: "off", serve: true }), members)).toBe("off");
  });

  it("is a BombVault when the address is on a paired instance", () => {
    expect(relayForm(relay({ url: "wss://192.168.1.21:8443/relay" }), members)).toBe("instance");
    expect(relayForm(relay({ url: "  https://192.168.1.21  " }), members)).toBe("instance");
  });

  it("is a BombVault when the address ends where an instance serves its relay", () => {
    expect(relayForm(relay({ url: "wss://vault.example.org/relay/connect" }), [])).toBe("instance");
    expect(relayForm(relay({ url: "wss://vault.example.org/relay/connect/" }), [])).toBe("instance");
  });

  it("is an own relay for any other address", () => {
    expect(relayForm(relay({ url: "wss://relay.example.org" }), members)).toBe("own");
    expect(relayForm(relay({ url: "wss://relay.example.org/connect", serve: true }), members)).toBe("own");
    expect(relayForm(relay({ url: "relay.example.org" }), members)).toBe("own");
  });

  it("goes by whether this instance serves a relay while no address is stored", () => {
    expect(relayForm(relay({}), members)).toBe("own");
    expect(relayForm(relay({ serve: true }), members)).toBe("instance");
  });
});
