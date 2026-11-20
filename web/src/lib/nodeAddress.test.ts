import { describe, expect, it } from "vitest";
import { nodeAddressProblem } from "./nodeAddress";

describe("nodeAddressProblem", () => {
  it.each(["https://backup.example.org", "https://203.0.113.7:8443/", "http://192.168.1.20:8000", "http://10.0.0.5", "http://172.20.1.1:8000", "http://100.101.102.103:8000", "http://[fd7a:115c:a1e0::1]:8000", "http://127.0.0.1:8000", ""])(
    "takes %s",
    (url) => {
      expect(nodeAddressProblem(url)).toBeNull();
    },
  );

  it.each(["http://backup.example.org:8000", "http://203.0.113.7:8000", "http://172.32.0.1", "http://100.128.0.1"])("refuses %s as unencrypted over the internet", (url) => {
    expect(nodeAddressProblem(url)).toBe("plain");
  });

  it.each(["backup.example.org", "ftp://192.168.1.20", "https://user:pw@backup.example.org", "https://backup.example.org/?a=1"])("refuses %s as no address", (url) => {
    expect(nodeAddressProblem(url)).toBe("form");
  });

  it("refuses a folder in the address of a node's own rest-server", () => {
    expect(nodeAddressProblem("https://203.0.113.7:8443/backups", true)).toBe("form");
    expect(nodeAddressProblem("https://203.0.113.7:8443/", true)).toBeNull();
    expect(nodeAddressProblem("http://203.0.113.7:8000", true)).toBe("plain");
  });
});
