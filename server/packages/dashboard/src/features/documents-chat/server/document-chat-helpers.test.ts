import { describe, expect, test } from "bun:test";
import { toJsonSchema } from "@langchain/core/utils/json_schema";
import { searchDocumentsToolInputSchema } from "@/features/documents-chat/server/document-chat-agent";
import {
	chunkAssistantText,
	dedupeCitations,
	extractLastAssistantText,
} from "@/features/documents-chat/server/document-chat-helpers";
import { collectionSearcherResponseSchema } from "@/features/documents-chat/types";

describe("extractLastAssistantText", () => {
	test("returns the newest assistant message text from mixed message shapes", () => {
		const text = extractLastAssistantText([
			{ role: "user", content: "hello" },
			{
				type: "ai",
				content: [
					{ text: "The first answer." },
					{ type: "text", content: "This should still work." },
				],
			},
			{
				role: "assistant",
				content: [
					{ type: "text", content: "Latest grounded answer." },
					{ type: "image", url: "ignored" },
				],
			},
		]);

		expect(text).toBe("Latest grounded answer.");
	});
});

describe("chunkAssistantText", () => {
	test("splits long text into readable streaming chunks", () => {
		const chunks = chunkAssistantText(
			"This answer is intentionally a little longer so we can verify that the assistant text is broken into multiple chunks instead of streaming as one giant block.",
		);

		expect(chunks.length).toBeGreaterThan(1);
		expect(chunks.join("")).toContain("assistant text is broken");
	});
});

describe("dedupeCitations", () => {
	test("keeps one citation per document and prefers the richer excerpt", () => {
		const citations = dedupeCitations([
			{
				documentId: "doc-1",
				projectId: "project-1",
				projectName: "Project One",
				label: "manual.png",
				excerpt: "Short excerpt",
				matchReason: "description",
			},
			{
				documentId: "doc-1",
				projectId: "project-1",
				projectName: "Project One",
				label: "manual.png",
				excerpt:
					"Longer and more useful excerpt that should win when citations are deduplicated.",
				matchReason: "OCR text",
			},
			{
				documentId: "doc-2",
				projectId: "project-1",
				projectName: "Project One",
				label: "diagram.png",
				excerpt: "Separate document excerpt",
				matchReason: "metadata",
			},
		]);

		expect(citations).toHaveLength(2);
		expect(citations[0]?.excerpt).toContain("Longer and more useful");
		expect(citations[1]?.documentId).toBe("doc-2");
	});
});

describe("OpenAI schema compatibility", () => {
	test("marks search tool limit as required in emitted JSON schema", () => {
		const schema = toJsonSchema(searchDocumentsToolInputSchema) as {
			required?: string[];
		};
		const required = Array.isArray(schema.required) ? schema.required : [];

		expect(required).toEqual(["query", "limit"]);
	});

	test("marks subagent response fields as required when nullable", () => {
		const schema = toJsonSchema(collectionSearcherResponseSchema) as {
			properties?: Record<string, unknown>;
		};
		const documentsSchema = schema.properties?.documents as
			| { items?: unknown }
			| undefined;
		const documentItemSchema = (
			documentsSchema &&
			typeof documentsSchema === "object" &&
			"items" in documentsSchema
				? documentsSchema.items
				: undefined
		) as { required?: string[] } | undefined;
		const required =
			documentItemSchema && Array.isArray(documentItemSchema.required)
				? documentItemSchema.required
				: [];

		expect(required).toContain("deviceName");
	});
});
