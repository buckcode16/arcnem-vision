import { describe, expect, test } from "bun:test";
import { chunkOCRText } from "./ocr-text";

describe("chunkOCRText", () => {
	test("keeps paragraphs together when they fit", () => {
		const chunks = chunkOCRText("First paragraph.\n\nSecond paragraph.", 80);
		expect(chunks).toEqual(["First paragraph.\n\nSecond paragraph."]);
	});

	test("splits long text into multiple chunks", () => {
		const text = `${"A".repeat(100)}\n\n${"B".repeat(100)}`;
		const chunks = chunkOCRText(text, 120);
		expect(chunks.length).toBeGreaterThan(1);
		expect(chunks.join("\n\n").replace(/\n{3,}/g, "\n\n")).toContain("AAAA");
		expect(chunks.join("\n\n")).toContain("BBBB");
	});
});
