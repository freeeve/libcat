import { describe, expect, it } from "vitest";
import { linkInfo } from "./links";

describe("linkInfo", () => {
  it("labels OverDrive title links", () => {
    expect(linkInfo("http://link.overdrive.com/?websiteID=173&titleID=798942")).toEqual({
      label: "OverDrive title page",
      image: false,
    });
  });

  it("labels OverDrive samples", () => {
    expect(linkInfo("https://samples.overdrive.com/?crid=8BF51CB6-247E-44E1-8FDD-B9229897A83C&.epub-sample.overdrive.com")).toEqual({
      label: "Sample (excerpt)",
      image: false,
    });
  });

  it("labels od-cdn covers and thumbnails as images", () => {
    expect(linkInfo("https://img1.od-cdn.com/ImageType-100/1095-1/%7B8BF51CB6-247E-44E1-8FDD-B9229897A83C%7DImg100.jpg")).toEqual({
      label: "Cover image",
      image: true,
    });
    expect(linkInfo("https://img1.od-cdn.com/ImageType-200/1095-1/%7B8BF51CB6-247E-44E1-8FDD-B9229897A83C%7DImg200.jpg")).toEqual({
      label: "Cover thumbnail",
      image: true,
    });
  });

  it("detects generic image links by extension", () => {
    expect(linkInfo("https://example.org/covers/big.PNG")).toEqual({ label: "Image", image: true });
    expect(linkInfo("https://example.org/covers/big.png?size=2")).toEqual({ label: "Image", image: true });
  });

  it("passes ordinary and malformed URLs through unlabeled", () => {
    expect(linkInfo("https://example.org/about")).toEqual({ label: "", image: false });
    expect(linkInfo("not a url")).toEqual({ label: "", image: false });
  });
});
