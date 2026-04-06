import type { BaseMessageLike } from "@langchain/core/messages";
import { type StreamChunk, toServerSentEventsResponse } from "@tanstack/ai";
import { DashboardMcpClient } from "@/features/documents-chat/server/dashboard-mcp-client";
import {
	createCitationSink,
	getDocumentChatAgent,
} from "@/features/documents-chat/server/document-chat-agent";
import {
	chunkAssistantText,
	dedupeCitations,
	extractLastAssistantText,
} from "@/features/documents-chat/server/document-chat-helpers";
import type { ChatScope } from "@/features/documents-chat/types";

type DocumentChatStreamOptions = {
	messages: BaseMessageLike[];
	conversationId: string;
	organizationId: string;
	userId: string;
	scope: ChatScope;
	signal: AbortSignal;
};

export function createDocumentChatResponse(options: DocumentChatStreamOptions) {
	return toServerSentEventsResponse(streamDocumentChatResponse(options));
}

async function* streamDocumentChatResponse(
	options: DocumentChatStreamOptions,
): AsyncGenerator<StreamChunk> {
	const runId = options.conversationId;
	const messageId = `assistant-${crypto.randomUUID()}`;
	const agent = getDocumentChatAgent();
	const mcpClient = new DashboardMcpClient();
	const citationSink = createCitationSink();
	const timestamp = () => Date.now();

	yield {
		type: "RUN_STARTED",
		runId,
		threadId: options.conversationId,
		timestamp: timestamp(),
	};

	try {
		const result = await agent.invoke(
			{ messages: options.messages },
			{
				context: {
					organizationId: options.organizationId,
					userId: options.userId,
					scope: options.scope,
					mcpClient,
					citationSink,
				},
				signal: options.signal,
				configurable: {
					thread_id: options.conversationId,
					run_id: runId,
				},
			},
		);

		const answer =
			extractLastAssistantText(result.messages) ||
			"I couldn't find enough grounded information in the current document collection to answer that confidently yet.";
		const citations = dedupeCitations(citationSink.citations).slice(0, 8);

		yield {
			type: "TEXT_MESSAGE_START",
			messageId,
			role: "assistant",
			timestamp: timestamp(),
		};

		for (const chunk of chunkAssistantText(answer)) {
			yield {
				type: "TEXT_MESSAGE_CONTENT",
				messageId,
				delta: chunk,
				timestamp: timestamp(),
			};
		}

		if (citations.length > 0) {
			yield {
				type: "CUSTOM",
				name: "assistant_sources",
				value: {
					messageId,
					citations,
				},
				timestamp: timestamp(),
			};
		}

		yield {
			type: "TEXT_MESSAGE_END",
			messageId,
			timestamp: timestamp(),
		};
		yield {
			type: "RUN_FINISHED",
			runId,
			finishReason: "stop",
			timestamp: timestamp(),
		};
	} catch (error) {
		yield {
			type: "RUN_ERROR",
			runId,
			error: {
				message:
					error instanceof Error
						? error.message
						: "Document chat failed unexpectedly.",
			},
			timestamp: timestamp(),
		};
	} finally {
		await mcpClient.close();
	}
}
