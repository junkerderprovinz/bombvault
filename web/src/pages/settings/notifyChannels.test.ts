import { describe, expect, it } from "vitest";
import { newChannel, channelTarget } from "./notifyChannels";

describe("where a channel sends", () => {
  it("names the address a service is reached at", () => {
    const c = newChannel("ntfy", []);
    c.ntfy!.url = "https://ntfy.example/backups";
    expect(channelTarget(c)).toBe("https://ntfy.example/backups");
  });

  it("names the room and its server for Matrix", () => {
    const c = newChannel("matrix", []);
    c.matrix = { homeserver: "https://matrix.example", room: "#backups:example", token: "" };
    expect(channelTarget(c)).toBe("#backups:example, https://matrix.example");
  });

  it("names the recipient of an email", () => {
    const c = newChannel("smtp", []);
    c.smtp!.to = "ops@example.org";
    c.smtp!.host = "mail.example.org";
    expect(channelTarget(c)).toBe("ops@example.org");
  });

  it("names the first Apprise address when the channel holds several", () => {
    const c = newChannel("apprise", []);
    c.apprise = { url: "", urls: ["tgram://a/b", "mailto://c"] };
    expect(channelTarget(c)).toBe("tgram://a/b");
  });

  it("names nothing for a channel whose address is still blank, and for Unraid", () => {
    expect(channelTarget(newChannel("discord", []))).toBe("");
    expect(channelTarget(newChannel("unraid", []))).toBe("");
  });

  it("leaves out the spaces around a typed address", () => {
    const c = newChannel("webhook", []);
    c.webhook!.url = "  https://hooks.example/in  ";
    expect(channelTarget(c)).toBe("https://hooks.example/in");
  });
});
