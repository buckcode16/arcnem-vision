import { getDB } from "@arcnem-vision/db/server";
import type { BaseMessageLike } from "@langchain/core/messages";
import {
	convertMessagesToModelMessages,
	type ModelMessage,
} from "@tanstack/ai";
import { createFileRoute } from "@tanstack/react-router";
import { AIMessage, HumanMessage, ToolMessage } from "langchain";
import { getSessionContext } from "@/features/dashboard/server/session-context";
import {
	type DocumentChatRequest,
	documentChatRequestSchema,
	resolveRequestedChatScope,
} from "@/features/documents-chat/server/document-chat-helpers";
import { createDocumentChatResponse } from "@/features/documents-chat/server/document-chat-stream";

export const Route = createFileRoute("/api/documents/chat")({
	server: {
		handlers: {
			POST: async ({ request }) => {
				const sessionContext = await getSessionContext();
				if (!sessionContext.session) {
					return new Response("Unauthorized", { status: 401 });
				}
				if (!sessionContext.organizationId) {
					return new Response("No organization context", { status: 403 });
				}

				let payload: unknown;
				try {
					payload = await request.json();
				} catch {
					return new Response("Invalid JSON body", { status: 400 });
				}

				const parsed = documentChatRequestSchema.safeParse(payload);
				if (!parsed.success) {
					return new Response("Invalid document chat request", {
						status: 400,
					});
				}

				try {
					const scope = await resolveRequestedChatScope(
						getDB(),
						sessionContext.organizationId,
						parsed.data.data?.scope ?? parsed.data.scope,
					);

					return createDocumentChatResponse({
						messages: toAgentMessages(parsed.data.messages),
						conversationId:
							parsed.data.data?.conversationId ??
							parsed.data.conversationId ??
							crypto.randomUUID(),
						organizationId: sessionContext.organizationId,
						userId: sessionContext.session.userId,
						scope,
						signal: request.signal,
					});
				} catch (error) {
					return new Response(
						error instanceof Error ? error.message : "Document chat failed",
						{ status: 400 },
					);
				}
			},
		},
	},
});

function toAgentMessages(
	messages: DocumentChatRequest["messages"],
): BaseMessageLike[] {
	return convertMessagesToModelMessages(messages as never).map((message) => {
		switch (message.role) {
			case "user":
				return new HumanMessage(modelMessageContentToString(message));
			case "assistant":
				return new AIMessage({
					content: modelMessageContentToString(message),
					name: message.name,
					tool_calls: message.toolCalls as never,
				});
			case "tool":
				return new ToolMessage({
					content: modelMessageContentToString(message),
					tool_call_id: message.toolCallId ?? crypto.randomUUID(),
				});
			default:
				return new HumanMessage(modelMessageContentToString(message));
		}
	});
}

function modelMessageContentToString(message: ModelMessage): string {
	if (typeof message.content === "string") {
		return message.content;
	}
	if (!Array.isArray(message.content)) {
		return "";
	}

	return message.content
		.map((part) => {
			if (part.type === "text") {
				return part.content;
			}
			if ("source" in part) {
				return `[${part.type}]`;
			}
			return "";
		})
		.filter(Boolean)
		.join("\n");
}
