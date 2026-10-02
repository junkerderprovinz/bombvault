import { describe, expect, it } from "vitest";
import { mainBackCamera } from "./QRScanner";

const camera = (deviceId: string, label: string) => ({ kind: "videoinput", deviceId, label }) as MediaDeviceInfo;

describe("mainBackCamera", () => {
  it("takes the back camera Android numbers first, not the telephoto the browser picks", () => {
    const devices = [
      camera("tele", "camera2 4, facing back"),
      camera("front", "camera2 1, facing front"),
      camera("main", "camera2 0, facing back"),
      camera("wide", "camera2 2, facing back"),
    ];
    expect(mainBackCamera(devices)).toBe("main");
  });

  it("leaves the choice to the browser when the labels name no back camera", () => {
    expect(mainBackCamera([camera("a", ""), camera("b", "camera2 1, facing front")])).toBeUndefined();
  });
});
