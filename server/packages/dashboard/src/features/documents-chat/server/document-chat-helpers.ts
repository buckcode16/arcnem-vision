import { devices, documents, projects } from "@arcnem-vision/db/schema";
import type { PGDB } from "@arcnem-vision/db/server";
import { and, count, eq, inArray } from "drizzle-orm";
import { z } from "zod";
import {
	type ChatScope,
	chatScopeSchema,
	type DocumentChatCitation,
} from "@/features/documents-chat/types";

export const documentChatRequestSchema = z.object({
	messages: z.array(z.unknown()).min(1),
	data: z
		.object({
			conversationId: z.string().min(1).optional(),
			scope: chatScopeSchema.optional(),
		})
		.optional(),
	conversationId: z.string().min(1).optional(),
	scope: chatScopeSchema.optional(),
});

export type DocumentChatRequest = z.infer<typeof documentChatRequestSchema>;

export async function resolveRequestedChatScope(
	db: PGDB,
	authOrganizationId: string,
	requestedScope?: ChatScope,
): Promise<ChatScope> {
	const scope = requestedScope ?? {
		kind: "organization",
		organizationId: authOrganizationId,
	};

	if (scope.organizationId !== authOrganizationId) {
		throw new Error("Requested scope does not match the active organization.");
	}

	const normalizedScope: ChatScope = {
		kind: "organization",
		organizationId: authOrganizationId,
		projectIds: dedupeIds(scope.projectIds),
		deviceIds: dedupeIds(scope.deviceIds),
		documentIds: dedupeIds(scope.documentIds),
	};

	await assertScopeIdsBelongToOrganization(
		normalizedScope.projectIds,
		"projectIds",
		async (ids) => {
			const [{ total }] = await db
				.select({ total: count() })
				.from(projects)
				.where(
					and(
						eq(projects.organizationId, authOrganizationId),
						inArray(projects.id, ids),
					),
				);

			return total;
		},
	);
	await assertScopeIdsBelongToOrganization(
		normalizedScope.deviceIds,
		"deviceIds",
		async (ids) => {
			const [{ total }] = await db
				.select({ total: count() })
				.from(devices)
				.where(
					and(
						eq(devices.organizationId, authOrganizationId),
						inArray(devices.id, ids),
					),
				);

			return total;
		},
	);
	await assertScopeIdsBelongToOrganization(
		normalizedScope.documentIds,
		"documentIds",
		async (ids) => {
			const [{ total }] = await db
				.select({ total: count() })
				.from(documents)
				.where(
					and(
						eq(documents.organizationId, authOrganizationId),
						inArray(documents.id, ids),
					),
				);

			return total;
		},
	);

	return normalizedScope;
}

async function assertScopeIdsBelongToOrganization(
	ids: string[] | undefined,
	label: string,
	countScopedRecords: (ids: string[]) => Promise<number>,
) {
	if (!ids?.length) {
		return;
	}

	const total = await countScopedRecords(ids);

	if (total !== ids.length) {
		throw new Error(`Requested ${label} are outside the active organization.`);
	}
}

function dedupeIds(ids?: string[]) {
	if (!ids?.length) {
		return undefined;
	}

	const deduped = Array.from(
		new Set(ids.map((id) => id.trim()).filter(Boolean)),
	);
	return deduped.length > 0 ? deduped : undefined;
}

export function extractLastAssistantText(messages: unknown): string {
	if (!Array.isArray(messages)) {
		return "";
	}

	for (let index = messages.length - 1; index >= 0; index -= 1) {
		const message = messages[index] as {
			role?: string;
			type?: string;
			content?: unknown;
			_getType?: () => string;
		};
		const messageType = message?._getType?.() ?? message?.type ?? message?.role;
		if (messageType !== "ai" && messageType !== "assistant") {
			continue;
		}

		return contentToPlainText(message.content);
	}

	return "";
}

function contentToPlainText(content: unknown): string {
	if (typeof content === "string") {
		return content;
	}
	if (!Array.isArray(content)) {
		return "";
	}

	return content
		.map((part) => {
			if (typeof part === "string") {
				return part;
			}
			if (!part || typeof part !== "object") {
				return "";
			}

			if ("text" in part && typeof part.text === "string") {
				return part.text;
			}
			if ("content" in part && typeof part.content === "string") {
				return part.content;
			}

			return "";
		})
		.filter(Boolean)
		.join("\n");
}

export function chunkAssistantText(text: string): string[] {
	const normalized = text.trim();
	if (!normalized) {
		return [];
	}

	const words = normalized.split(/\s+/);
	const chunks: string[] = [];
	let currentChunk = "";

	for (const word of words) {
		const candidate = currentChunk ? `${currentChunk} ${word}` : word;
		if (candidate.length > 80 && currentChunk) {
			chunks.push(`${currentChunk} `);
			currentChunk = word;
			continue;
		}
		currentChunk = candidate;
	}

	if (currentChunk) {
		chunks.push(currentChunk);
	}

	return chunks;
}

export function dedupeCitations(
	citations: DocumentChatCitation[],
): DocumentChatCitation[] {
	const deduped = new Map<string, DocumentChatCitation>();

	for (const citation of citations) {
		if (!deduped.has(citation.documentId)) {
			deduped.set(citation.documentId, citation);
			continue;
		}

		const current = deduped.get(citation.documentId);
		if (!current) {
			continue;
		}

		if (
			citation.excerpt.length > current.excerpt.length &&
			citation.excerpt.length <= 280
		) {
			deduped.set(citation.documentId, citation);
		}
	}

	return [...deduped.values()];
}
